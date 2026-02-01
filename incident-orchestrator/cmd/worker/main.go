package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"incident-orchestrator/internal/activities"
	"incident-orchestrator/internal/workflows"
)

const TaskQueue = "incident-task-queue"

func main() {
	log.Println("Starting incident orchestrator worker...")

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal client:", err)
	}
	defer c.Close()

	// Create worker
	w := worker.New(c, TaskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(workflows.IncidentWorkflow)

	// Register activities
	w.RegisterActivity(activities.SendNotification)

	log.Printf("Worker listening on task queue: %s", TaskQueue)

	// Start worker (blocks until interrupted)
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker:", err)
	}
}
