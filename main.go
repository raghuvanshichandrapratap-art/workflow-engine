package main

import (
	// "fmt"
	"log"

	// "github.com/gin-gonic/gin"

	// "database/sql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Workflow struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// var workflows = []Workflow{
// 	{ID: 1, Name: "Age Validation", Description: "Reject under 18"},
// 	{ID: 2, Name: "Email Verification", Description: "Send verification email"},
// }

// func addWorkflow(c *gin.Context) {
// 	var newWorkflow Workflow
// 	if err := c.BindJSON(&newWorkflow); err != nil {
// 		c.JSON(400, gin.H{"error": err.Error()})
// 		return
// 	}
// 	workflows = append(workflows, newWorkflow)
// 	c.IndentedJSON(201, newWorkflow)
// }

// func getWorkflow(c *gin.Context) {
// 	id := c.Param("id")
// 	workflow, err := getWorkflowbyId(id)
// 	if err != nil {
// 		c.IndentedJSON(404, gin.H{"error": err.Error()})
// 		return
// 	}
// 	c.IndentedJSON(200, workflow)
// }

// func getWorkflowbyId(id string) (*Workflow, error) {
// 	for _, w := range workflows {
// 		if fmt.Sprintf("%d", w.ID) == id {
// 			return &w, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("Workflow with ID %s not found", id)
// }

// func deleteWorkflow(c *gin.Context) {
// 	id := c.Param("id")
// 	if err := deleteWorkflowById(id); err != nil {
// 		c.IndentedJSON(404, gin.H{"error": err.Error()})
// 		return
// 	}
// 	c.Status(204)
// }

// func deleteWorkflowById(id string) error {
// 	for i, w := range workflows {
// 		if fmt.Sprintf("%d", w.ID) == id {
// 			workflows = append(workflows[:i], workflows[i+1:]...)
// 			return nil
// 		}
// 	}
// 	return fmt.Errorf("Workflow with ID %s not found", id)
// }

func main() {
	// router := gin.Default()
	// router.POST("/workflow", addWorkflow)
	// router.GET("/workflow/:id", getWorkflow)
	// router.DELETE("/workflow/:id", deleteWorkflow)
	// router.Run() // listens on 0.0.0.0:8080 by default
	dsn := "host=localhost user=postgres password= dbname=workflow_engine port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&Workflow{})

	log.Println("Connected!")

	// create
	workflow := Workflow{
		Name:        "Email verification",
		Description: "Send verification email",
	}
	db.Create(&workflow)

	// read
	var readWorkflow Workflow
	db.First(&readWorkflow, workflow.ID)
	log.Printf("Read Workflow: %+v\n", readWorkflow)

	// update
	db.Model(&readWorkflow).Update("Description", "Reject under 21")
	log.Printf("Updated Workflow: %+v\n", readWorkflow)

	// delete
	db.Delete(&readWorkflow)
	log.Println("Deleted Workflow")
}
