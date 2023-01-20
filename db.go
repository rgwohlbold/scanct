package main

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
	Name      string
	Index     int64 `gorm:"index:index_index,unique"`
	Processed bool
}

type GitLab struct {
	ID          int
	InstanceID  int
	Instance    Instance `gorm:"foreignKey:InstanceID"`
	AllowSignup bool
	Email       string
	Password    string
	Processed   bool
}

type Repository struct {
	ID        int
	GitLabID  int
	GitLab    GitLab `gorm:"foreignKey:GitLabID"`
	Name      string
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
	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		return Database{}, errors.Wrap(err, "could not open database")
	}
	err = db.AutoMigrate(&Instance{}, &GitLab{}, &Repository{}, &Finding{})
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

func (d *Database) GetUnprocessedPotentialGitLabs() ([]Instance, error) {
	var instances []Instance
	err := d.db.Where("processed = false").Where("name like 'gitlab.%'").Find(&instances).Error
	if err != nil {
		return nil, errors.Wrap(err, "could not get unprocessed instances")
	}
	return instances, nil
}

func (d *Database) AddGitLab(g GitLab) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Create(&g).Error
		if err != nil {
			return err
		}
		return tx.Where("id = ?", g.InstanceID).Set("processed", true).Error
	})
}

func (d *Database) SetGitlabProcessed(gitlabID int) error {
	return d.db.Table("git_labs").Where("id = ?", gitlabID).Update("processed", true).Error
}

func (d *Database) SetInstanceProcessed(instanceID int) error {
	return d.db.Table("instances").Where("id = ?", instanceID).Update("processed", true).Error
}

func (d *Database) StoreCertificates(certs []Certificate) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, cert := range certs {
			for _, subject := range cert.Subjects {
				instance := Instance{Index: cert.Index, Name: subject, Processed: false}
				err := tx.Save(&instance).Error
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (d *Database) LogFinding(f *GitFinding) error {
	if len(f.Finding.Secret) > 50 {
		f.Secret = fmt.Sprint(f.Secret[:50], "...")
	}
	url := fmt.Sprintf("https://%s/%s#L%d-%d", f.Repository.GitLab.Instance.Name, f.File, f.StartLine, f.EndLine)
	finding := Finding{
		RepositoryID: f.Repository.ID,
		Secret:       f.Secret,
		Commit:       f.Commit,
		StartLine:    f.StartLine,
		EndLine:      f.EndLine,
		File:         f.File,
		URL:          url,
	}
	err := d.db.Save(&finding).Error
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

func (g *GitLab) URL() string {
	return fmt.Sprintf("https://%s/api/v4", g.Instance.Name)
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
