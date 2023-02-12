package scanct

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	stdlog "log"
	"math"
	"os"
	"time"
)

type Database struct {
	db *gorm.DB
}

type Instance struct {
	ID        int
	Name      string `gorm:"index:index_name"`
	Index     int64  `gorm:"index:index_index"`
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
	Processed    bool
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
	Processed bool
}

type AWSKey struct {
	ID               int
	AccessKey        string `gorm:"uniqueIndex:accesskey"`
	SecretKey        string
	FindingID        int
	Finding          Finding `gorm:"foreignKey:FindingID"`
	JenkinsFindingID int
	JenkinsFinding   JenkinsFinding `gorm:"foreignKey:JenkinsFindingID"`
	Arn              string
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
	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{
		Logger: logger.New(stdlog.New(os.Stdout, "\r\n", stdlog.LstdFlags), logger.Config{
			SlowThreshold: time.Second,
		}),
	})
	//db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		return Database{}, errors.Wrap(err, "could not open database")
	}
	err = db.AutoMigrate(&Instance{}, &GitLab{}, &Jenkins{}, &JenkinsJob{}, &Repository{}, &Finding{}, &JenkinsFinding{}, &AWSKey{})
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

	var instance Instance
	err := d.db.Limit(1).Find(&instance).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get count")
	}
	if instance.ID == 0 {
		return math.MaxInt64 / 2, math.MaxInt64 / 2, nil
	}

	var minIndex, maxIndex int64
	err = d.db.Raw("select max(\"index\") from instances").Scan(&maxIndex).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get index range")
	}
	err = d.db.Raw("select min(\"index\") from instances").Scan(&minIndex).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get index range")
	}
	return minIndex, maxIndex, nil
}

func (d *Database) GetUnprocessedInstancesForGitlab() ([]Instance, error) {
	var instances []Instance
	err := d.db.Where("processed = false").Where("name between 'gitlab.' and 'gitlab/'").Where("name not like 'gitlab.git%'").Find(&instances).Error
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

func (d *Database) AddGitLab(g []GitLab) error {
	return d.db.Clauses(clause.OnConflict{DoNothing: true}).Save(g).Error
}

func (d *Database) AddJenkins(j []Jenkins) error {
	return d.db.Clauses(clause.OnConflict{DoNothing: true}).Create(j).Error
}

func (d *Database) SetGitlabProcessed(gitlab *GitLab) error {
	return d.db.Table("git_labs").Where("id = ?", gitlab.ID).Update("processed", true).Error
}

func (d *Database) SetInstanceProcessed(instance *Instance) error {
	return d.db.Table("instances").Where("id = ?", instance.ID).Update("processed", true).Error
}

func (d *Database) SetRepositoryProcessed(repository *Repository) error {
	return d.db.Table("repositories").Where("id = ?", repository.ID).Update("processed", true).Error
}

func (d *Database) StoreCertificates(certs []Certificate) error {
	instances := make([]Instance, 0, len(certs))
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

func (d *Database) LogFindings(finding []Finding) error {
	return d.db.Save(&finding).Error
}

func (d *Database) GetUnprocessedGitLabs() ([]GitLab, error) {
	var gl []GitLab
	err := d.db.Where("processed = false").Preload(clause.Associations).Find(&gl).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed repositories")
	}
	return gl, nil
}

func (d *Database) InsertRepositories(repositories []Repository) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, repo := range repositories {
			err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&repo).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Database) GetUnprocessedJenkins() ([]Jenkins, error) {
	var j []Jenkins
	err := d.db.Where("processed = false").Preload(clause.Associations).Find(&j).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed jenkins")
	}
	return j, nil
}

func (d *Database) SetJenkinsProcessed(jenkins *Jenkins) error {
	return d.db.Table("jenkins").Where("id = ?", jenkins.ID).Update("processed", true).Error
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

func (d *Database) GetUnprocessedAWSFindings() ([]Finding, error) {
	var findings []Finding
	query := d.db.Where("processed = false")
	query = query.Where("rule = 'aws-access-token'")
	query = query.Where("file not like '%gltf'")
	query = query.Where("file not like '%ipynb'")
	query = query.Where("file not like '%json'")
	query = query.Where("file not like '%UPID_sequences_human.json'")
	query = query.Where("secret not like '%AAAAAA%'")
	query = query.Where("start_line < 1000")
	err := query.Preload("Repository").Preload("Repository.GitLab").Find(&findings).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed aws keys")
	}
	return findings, nil
}

func (d *Database) GetUnprocessedJenkinsAWSFindings() ([]JenkinsFinding, error) {
	var findings []JenkinsFinding
	query := d.db.Where("processed = false")
	query = query.Where("rule = 'aws-access-token'")
	query = query.Where("file not like '%gltf'")
	query = query.Where("file not like '%ipynb'")
	query = query.Where("file not like '%json'")
	query = query.Where("file not like '%UPID_sequences_human.json'")
	query = query.Where("secret not like '%AAAAAA%'")
	query = query.Where("start_line < 1000")
	err := query.Preload("Job").Preload("Job.Jenkins").Find(&findings).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed aws keys")
	}
	return findings, nil
}

func (d *Database) SetFindingProcessed(finding *Finding) error {
	return d.db.Table("findings").Where("id = ?", finding.ID).Update("processed", true).Error
}

func (d *Database) AddAWSKeys(k []AWSKey) error {
	return d.db.Clauses(clause.OnConflict{DoNothing: true}).Save(&k).Error
}

func (d *Database) SetJenkinsFindingProcessed(finding *JenkinsFinding) error {
	return d.db.Table("jenkins_findings").Where("id = ?", finding.ID).Update("processed", true).Error
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
