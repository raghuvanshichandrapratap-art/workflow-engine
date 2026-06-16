package main

import (
	"workflow-engine/config"
	"workflow-engine/handlers"
	"workflow-engine/repos"
	"workflow-engine/services"

	"github.com/gin-gonic/gin"
)

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

	err := config.LoadEnv()
	if err != nil {
		panic("Failed to load environment variables")
	}

	db := config.ConnectDB(
		config.GetEnv("DB_HOST"),
		config.GetEnv("DB_USER"),
		config.GetEnv("DB_PASSWORD"),
		config.GetEnv("DB_NAME"),
		config.GetEnv("DB_PORT"),
	)

	repo := repos.NewWorkflowRepository(db)

	service := services.NewWorkflowService(repo)

	handler := handlers.NewWorkflowHandler(service)

	router := gin.Default()

	router.POST("/workflow", handler.CreateWorkflow)

	// repo := repos.WorkflowRepository{db}

	// // create
	// workflow := Workflow{
	// 	Name:        "Email verification",
	// 	Description: "Send verification email",
	// }
	// db.Create(&workflow)

	// // read
	// var readWorkflow Workflow
	// db.First(&readWorkflow, workflow.ID)
	// log.Printf("Read Workflow: %+v\n", readWorkflow)

	// // update
	// db.Model(&readWorkflow).Update("Description", "Reject under 21")
	// log.Printf("Updated Workflow: %+v\n", readWorkflow)

	// // delete
	// db.Delete(&readWorkflow)
	// log.Println("Deleted Workflow")
}
