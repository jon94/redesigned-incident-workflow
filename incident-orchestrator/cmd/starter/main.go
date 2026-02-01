package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"go.temporal.io/sdk/client"

	"incident-orchestrator/internal/models"
	"incident-orchestrator/internal/workflows"
)

const TaskQueue = "incident-task-queue"

func main() {
	// Parse command line flags
	command := flag.String("cmd", "", "Command: start, alert, ack, resolve, status")
	service := flag.String("service", "", "Service name (required for start)")
	alertID := flag.String("alert", "", "Alert ID (for alert command)")
	responder := flag.String("responder", "", "Responder name (for ack/resolve)")
	flag.Parse()

	if *command == "" {
		printUsage()
		os.Exit(1)
	}

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal client:", err)
	}
	defer c.Close()

	ctx := context.Background()

	switch *command {
	case "start":
		startWorkflow(ctx, c, *service)
	case "alert":
		sendAddAlert(ctx, c, *service, *alertID)
	case "ack":
		sendAck(ctx, c, *service, *responder)
	case "resolve":
		sendResolve(ctx, c, *service, *responder)
	case "status":
		queryStatus(ctx, c, *service)
	default:
		fmt.Printf("Unknown command: %s\n", *command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Incident Orchestrator CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  Start a new incident:")
	fmt.Println("    go run cmd/starter/main.go -cmd=start -service=<service-name>")
	fmt.Println()
	fmt.Println("  Add an alert to an incident:")
	fmt.Println("    go run cmd/starter/main.go -cmd=alert -service=<service-name> -alert=<alert-id>")
	fmt.Println()
	fmt.Println("  Acknowledge an incident:")
	fmt.Println("    go run cmd/starter/main.go -cmd=ack -service=<service-name> -responder=<name>")
	fmt.Println()
	fmt.Println("  Resolve an incident:")
	fmt.Println("    go run cmd/starter/main.go -cmd=resolve -service=<service-name> -responder=<name>")
	fmt.Println()
	fmt.Println("  Query incident status:")
	fmt.Println("    go run cmd/starter/main.go -cmd=status -service=<service-name>")
}

func workflowID(service string) string {
	return fmt.Sprintf("incident-%s", service)
}

func startWorkflow(ctx context.Context, c client.Client, service string) {
	if service == "" {
		log.Fatalln("Service name is required for start command")
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID(service),
		TaskQueue: TaskQueue,
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflows.IncidentWorkflow, service)
	if err != nil {
		log.Fatalln("Unable to start workflow:", err)
	}

	log.Printf("Started incident workflow for service: %s", service)
	log.Printf("  Workflow ID: %s", we.GetID())
	log.Printf("  Run ID: %s", we.GetRunID())
}

func sendAddAlert(ctx context.Context, c client.Client, service, alertID string) {
	if service == "" || alertID == "" {
		log.Fatalln("Service name and alert ID are required for alert command")
	}

	signal := models.AddAlertSignal{AlertID: alertID}
	err := c.SignalWorkflow(ctx, workflowID(service), "", workflows.SignalAddAlert, signal)
	if err != nil {
		log.Fatalln("Unable to send AddAlert signal:", err)
	}

	log.Printf("Sent AddAlert signal: alertID=%s to incident-%s", alertID, service)
}

func sendAck(ctx context.Context, c client.Client, service, responder string) {
	if service == "" || responder == "" {
		log.Fatalln("Service name and responder are required for ack command")
	}

	signal := models.AckSignal{Responder: responder}
	err := c.SignalWorkflow(ctx, workflowID(service), "", workflows.SignalAck, signal)
	if err != nil {
		log.Fatalln("Unable to send Ack signal:", err)
	}

	log.Printf("Sent Ack signal: responder=%s to incident-%s", responder, service)
}

func sendResolve(ctx context.Context, c client.Client, service, responder string) {
	if service == "" || responder == "" {
		log.Fatalln("Service name and responder are required for resolve command")
	}

	signal := models.ResolveSignal{Responder: responder}
	err := c.SignalWorkflow(ctx, workflowID(service), "", workflows.SignalResolve, signal)
	if err != nil {
		log.Fatalln("Unable to send Resolve signal:", err)
	}

	log.Printf("Sent Resolve signal: responder=%s to incident-%s", responder, service)
}

func queryStatus(ctx context.Context, c client.Client, service string) {
	if service == "" {
		log.Fatalln("Service name is required for status command")
	}

	response, err := c.QueryWorkflow(ctx, workflowID(service), "", workflows.QueryState)
	if err != nil {
		log.Fatalln("Unable to query workflow:", err)
	}

	var state models.IncidentState
	if err := response.Get(&state); err != nil {
		log.Fatalln("Unable to decode query result:", err)
	}

	output, _ := json.MarshalIndent(state, "", "  ")
	fmt.Println("Incident State:")
	fmt.Println(string(output))
}
