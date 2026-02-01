#!/bin/bash
# Development helper script

set -e

cd "$(dirname "$0")/.."

case "$1" in
    deps)
        echo "Downloading dependencies..."
        go mod tidy
        go mod download
        ;;
    worker)
        echo "Starting worker..."
        go run cmd/worker/main.go
        ;;
    start)
        if [ -z "$2" ]; then
            echo "Usage: ./scripts/dev.sh start <service-name>"
            exit 1
        fi
        echo "Starting incident for service: $2"
        go run cmd/starter/main.go -cmd=start -service="$2"
        ;;
    alert)
        if [ -z "$2" ] || [ -z "$3" ]; then
            echo "Usage: ./scripts/dev.sh alert <service-name> <alert-id>"
            exit 1
        fi
        go run cmd/starter/main.go -cmd=alert -service="$2" -alert="$3"
        ;;
    ack)
        if [ -z "$2" ] || [ -z "$3" ]; then
            echo "Usage: ./scripts/dev.sh ack <service-name> <responder>"
            exit 1
        fi
        go run cmd/starter/main.go -cmd=ack -service="$2" -responder="$3"
        ;;
    resolve)
        if [ -z "$2" ] || [ -z "$3" ]; then
            echo "Usage: ./scripts/dev.sh resolve <service-name> <responder>"
            exit 1
        fi
        go run cmd/starter/main.go -cmd=resolve -service="$2" -responder="$3"
        ;;
    status)
        if [ -z "$2" ]; then
            echo "Usage: ./scripts/dev.sh status <service-name>"
            exit 1
        fi
        go run cmd/starter/main.go -cmd=status -service="$2"
        ;;
    demo)
        echo "Running demo scenario..."
        echo ""
        echo "1. Starting incident for 'payment-api'..."
        go run cmd/starter/main.go -cmd=start -service=payment-api
        sleep 1
        
        echo ""
        echo "2. Adding alerts..."
        go run cmd/starter/main.go -cmd=alert -service=payment-api -alert=ALERT-001
        go run cmd/starter/main.go -cmd=alert -service=payment-api -alert=ALERT-002
        sleep 1
        
        echo ""
        echo "3. Checking status..."
        go run cmd/starter/main.go -cmd=status -service=payment-api
        sleep 1
        
        echo ""
        echo "4. Acknowledging incident..."
        go run cmd/starter/main.go -cmd=ack -service=payment-api -responder=alice
        sleep 1
        
        echo ""
        echo "5. Final status before resolve..."
        go run cmd/starter/main.go -cmd=status -service=payment-api
        sleep 1
        
        echo ""
        echo "6. Resolving incident..."
        go run cmd/starter/main.go -cmd=resolve -service=payment-api -responder=alice
        
        echo ""
        echo "Demo complete!"
        ;;
    *)
        echo "Incident Orchestrator Development Script"
        echo ""
        echo "Usage: ./scripts/dev.sh <command> [args]"
        echo ""
        echo "Commands:"
        echo "  deps                        Download Go dependencies"
        echo "  worker                      Start the Temporal worker"
        echo "  start <service>             Start a new incident"
        echo "  alert <service> <alert-id>  Add an alert to an incident"
        echo "  ack <service> <responder>   Acknowledge an incident"
        echo "  resolve <service> <resp>    Resolve an incident"
        echo "  status <service>            Query incident status"
        echo "  demo                        Run a demo scenario"
        ;;
esac
