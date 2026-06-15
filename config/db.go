package config

import (
	"log"
	// "database/sql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Workflow struct {
	ID          uint
	Name        string
	Description string
}

func ConnectDB() *gorm.DB {
	dsn := "host=localhost user=postgres password= dbname=workflow_engine port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&Workflow{})

	log.Println("Connected!")

	return db
}
