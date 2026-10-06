# Local workflow on one machine. Needs docker, minikube, kubectl, helm and terraform.
SIBLINGS := claims-rag-java claims-rag-python mcp-devplatform-guardrails
TF       := terraform -chdir=terraform/environments/minikube
PROFILE  ?= dev

.PHONY: cluster images lint apply destroy mcp-token status
cluster:
	minikube start --driver=docker --cpus=4 --memory=5g

# Build each service image from its own repo (../<name>) and load it into minikube.
images:
	for s in $(SIBLINGS); do docker build -t $$s:dev ../$$s && minikube image load $$s:dev; done

lint:
	@for s in services/*.yaml; do for p in dev prod-like; do \
	  helm lint charts/service -f charts/service/profiles/$$p.yaml -f $$s || exit 1; done; done
	terraform fmt -check -recursive terraform
	$(TF) init -backend=false >/dev/null && $(TF) validate

apply:
	$(TF) init && $(TF) apply -auto-approve -var profile=$(PROFILE)

destroy:
	$(TF) destroy -auto-approve

mcp-token:
	@kubectl -n apps get secret mcp-guardrails-tokens -o jsonpath='{.data.GUARDRAILS_TOKENS}' | base64 -d | sed 's/^local=//'; echo

status:
	kubectl get pods -A | grep -E "apps|monitoring"
