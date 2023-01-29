package scanct

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/xanzy/go-gitlab"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"os"
)

type Database struct {
	db *gorm.DB
}

type Instance struct {
	ID        int
	Name      string `gorm:"index:index_name"`
	Index     int64  `gorm:"index:index_index,unique"`
	Processed bool
}

type GitLab struct {
	ID          int
	InstanceID  int
	Instance    Instance `gorm:"foreignKey:InstanceID"`
	AllowSignup bool
	Email       string
	Password    string
	APIToken    string
	Processed   bool
	BaseURL     string `gorm:"uniqueIndex:git_labs_base_url"`
}

func (g GitLab) GetInstanceID() int {
	return g.InstanceID
}

type Jenkins struct {
	ID           int
	InstanceID   int
	Instance     Instance `gorm:"foreignKey:InstanceID"`
	AnonymousAPI bool
	BaseURL      string `gorm:"uniqueIndex:jenkins_base_url"`
	Processed    bool
	ScriptAccess bool
}

type JenkinsJob struct {
	ID        int
	JenkinsID int
	Jenkins   Jenkins `gorm:"foreignKey:JenkinsID"`
	Name      string
	URL       string `gorm:"uniqueIndex:jenkins_jobs_url"`
	Processed bool
}

func (j Jenkins) GetInstanceID() int {
	return j.InstanceID
}

type Repository struct {
	ID        int
	GitLabID  int    `gorm:"uniqueIndex:repo"`
	GitLab    GitLab `gorm:"foreignKey:GitLabID"`
	Name      string `gorm:"uniqueIndex:repo"`
	Processed bool
}

type Finding struct {
	ID           int
	RepositoryID int
	Repository   Repository `gorm:"foreignKey:RepositoryID"`
	Secret       string
	Commit       string
	StartLine    int
	EndLine      int
	File         string
	URL          string
	CommitDate   string
	Rule         string
}

type JenkinsFinding struct {
	ID        int
	JobID     int
	Job       JenkinsJob `gorm:"foreignKey:JobID"`
	Secret    string
	StartLine int
	EndLine   int
	File      string
	URL       string
	Rule      string
}

const DatabaseFile = "./instances.db"

func NewDatabase() (Database, error) {
	if _, err := os.Stat(DatabaseFile); errors.Is(err, os.ErrNotExist) {
		var f *os.File
		f, err = os.Create(DatabaseFile)
		if err != nil {
			log.Fatal().Err(err).Msg("could not create database")
		}
		_ = f.Close()

	}
	//db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{
	//	Logger: logger.Default.LogMode(logger.Info),
	//})
	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		return Database{}, errors.Wrap(err, "could not open database")
	}
	err = db.AutoMigrate(&Instance{}, &GitLab{}, &Jenkins{}, &JenkinsJob{}, &Repository{}, &Finding{}, &JenkinsFinding{})
	if err != nil {
		return Database{}, errors.Wrap(err, "could not open migrate instance")
	}
	//_, err = db.Exec("pragma synchronous = NORMAL")
	//_, err = db.Exec("pragma journal_mode=wal")
	return Database{db: db}, nil
}

func (d *Database) Close() {
	db, err := d.db.DB()
	if err != nil {
		log.Error().Err(err).Msg("error closing database")
	}
	err = db.Close()
	if err != nil {
		log.Error().Err(err).Msg("error closing database")
	}
}

func (d *Database) IndexRange() (int64, int64, error) {
	var rows int64
	err := d.db.Raw("select count(*) from instances").Scan(&rows).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not count instances")
	}
	if rows == 0 {
		return math.MaxInt64 / 2, math.MaxInt64 / 2, nil
	}

	type MaxMinRes struct {
		M1 int64
		M2 int64
	}
	var res MaxMinRes
	err = d.db.Raw("select max(\"index\") as m1, min(\"index\") as m2 from instances").Scan(&res).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get index range")
	}
	return res.M2, res.M1, nil
}

func (d *Database) GetUnprocessedInstancesForGitlab() ([]Instance, error) {
	var instances []Instance
	err := d.db.Where("processed = false").Where("name between 'gitlab.' and 'gitlab./'").Where("name not like 'gitlab.git%'").Find(&instances).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed instances")
	}
	return instances, nil
}

func (d *Database) GetUnprocessedInstancesForJenkins() ([]Instance, error) {
	var instances []Instance
	err := d.db.Where("processed = false").Where("name between 'jenkins.' and 'jenkins/'").Find(&instances).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed instances")
	}
	return instances, nil
}

func (d *Database) GetUnprocessedRepositories() ([]Repository, error) {
	var repos []Repository
	err := d.db.Where("processed = false").Preload(clause.Associations).Find(&repos).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed repositories")
	}
	return repos, nil
}

func (d *Database) AddGitLab(g GitLab) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&g).Error
		if err != nil {
			return err
		}
		return tx.Where("id = ?", g.InstanceID).Set("processed", true).Error
	})
}

func (d *Database) AddJenkins(j []Jenkins) error {
	return d.db.Clauses(clause.OnConflict{DoNothing: true}).Create(j).Error
}

func (d *Database) SetGitlabProcessed(gitlabID int) error {
	return d.db.Table("git_labs").Where("id = ?", gitlabID).Update("processed", true).Error
}

func (d *Database) SetInstanceProcessed(instanceID int) error {
	return d.db.Table("instances").Where("id = ?", instanceID).Update("processed", true).Error
}

func (d *Database) SetRepositoryProcessed(repositoryID int) error {
	return d.db.Table("repositories").Where("id = ?", repositoryID).Update("processed", true).Error
}

func (d *Database) StoreCertificates(certs []Certificate) error {
	instances := make([]Instance, len(certs))
	for _, cert := range certs {
		for _, subject := range cert.Subjects {
			instances = append(instances, Instance{
				Name:      subject,
				Index:     cert.Index,
				Processed: false,
			})
		}
	}
	return d.db.Create(&instances).Error
}

func (d *Database) LogFinding(finding *Finding) error {
	err := d.db.Save(finding).Error
	if err != nil {
		return errors.Wrap(err, "could not insert finding")
	}
	return nil
}

func (d *Database) GetUnprocessedGitLabs() ([]GitLab, error) {
	var gl []GitLab
	err := d.db.Where("processed = false").Preload(clause.Associations).Find(&gl).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed repositories")
	}
	return gl, nil
}

func (d *Database) InsertProjects(gitlab *GitLab, projects []*gitlab.Project) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, r := range projects {
			repo := Repository{
				GitLabID:  gitlab.ID,
				Name:      r.PathWithNamespace,
				Processed: false,
			}
			err := tx.Save(&repo).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Database) GetUnprocessedJenkins() ([]Jenkins, error) {
	var j []Jenkins
	err := d.db.Where("processed = false").Where("base_url = 'https://jenkins.worqcompany.net'").Preload(clause.Associations).Find(&j).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed jenkins")
	}
	return j, nil
}

func (d *Database) SetJenkinsProcessed(id int) error {
	return d.db.Table("jenkins").Where("id = ?", id).Update("processed", true).Error
}

func (d *Database) AddJenkinsJob(o []JenkinsJob) error {
	return d.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&o).Error
}

func (d *Database) GetUnprocessedJenkinsJobs() ([]JenkinsJob, error) {
	var j []JenkinsJob
	err := d.db.Where("processed = false").Preload(clause.Associations).Find(&j).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed jenkins")
	}
	return j, nil
}

func (d *Database) SaveJenkinsFindings(findings []JenkinsFinding) error {
	return d.db.Create(&findings).Error
}

func (d *Database) SetJenkinsJobProcessed(job *JenkinsJob) error {
	return d.db.Table("jenkins_jobs").Where("id = ?", job.ID).Update("processed", true).Error
}

func (g GitLab) URL() string {
	return fmt.Sprintf("%s/api/v4", g.BaseURL)
}

func (r *Repository) CloneURL() string {
	return fmt.Sprintf("%s/%s", r.GitLab.BaseURL, r.Name)
}

//func (d *Database) AddGitlabIfNotExists(g *GitlabInstance) error {
//	stmt, err := d.db.Prepare("insert into instance(ind, name, processed) values(null, ?, false) where not exists (select 1 from instance where name = ?)")
//	if err != nil {
//		return errors.Wrap(err, "could not prepare statement")
//	}
//	_, err = stmt.Exec(g.Domain)
//	if err != nil {
//		return errors.Wrap(err, "could not insert instance")
//	}
//	stmt, err = d.db.Prepare("insert into gitlab(instance_id, allow_signup, email, pass, processed) values((select id from instance where name = ?), false, '', '', false) where not exists (select 1 from instance where name = ? join gitlab on gitlab.instance_id = instance.id")
//	if err != nil {
//		return errors.Wrap(err, "could not prepare statement")
//	}
//	_, err = stmt.Exec(g.Domain)
//	if err != nil {
//		return errors.Wrap(err, "could not insert instance")
//	}
//	return nil
//
//}
