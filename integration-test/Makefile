# Integration test Makefile
COMPOSE_PROJECT := kubectl-rundeck-nodes-test
COMPOSE_FILE := docker-compose.yml

.PHONY: test clean logs status

# Run the full integration test
test:
	./scripts/run-test.sh

# Tear down all containers and volumes
clean:
	docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE) down -v 2>/dev/null || true

# Show container logs (useful for debugging)
logs:
	docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE) logs

# Show container status
status:
	docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE) ps
