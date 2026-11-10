# Makefile for url-shortener

IMAGE := url-shortener:latest

.PHONY: build deploy undeploy logs status port-forward test restart clean minikube-load minikube-delete loadtest loadtest-internal rolling-test

build:
	docker build -t $(IMAGE) .

deploy:
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/secret.yaml
	kubectl apply -f k8s/postgres.yaml
	kubectl apply -f k8s/redis.yaml
	kubectl apply -f k8s/service.yaml
	kubectl apply -f k8s/deployment.yaml

undeploy:
	kubectl delete -f k8s/ --ignore-not-found=true

logs:
	kubectl logs -l app=url-shortener-pod -f

logs-postgres:
	kubectl logs -l app=postgres -f

logs-redis:
	kubectl logs -l app=redis -f

status:
	kubectl get pods -l app=url-shortener-pod
	kubectl get pods -l app=postgres
	kubectl get pods -l app=redis
	kubectl get svc

port-forward:
	kubectl port-forward svc/url-shortener-svc 8080:80

test:
	kubectl port-forward svc/url-shortener-svc 8080:80 >/dev/null 2>&1 & \
	sleep 2 && \
	curl -s http://localhost:8080/healthz && \
	curl -s http://localhost:8080/readyz && \
	pkill -f "port-forward.*url-shortener-svc"

restart:
	kubectl rollout restart deployment/url-shortener-deployment
	kubectl rollout status deployment/url-shortener-deployment --timeout=120s

minikube-load:
	minikube image load $(IMAGE)

minikube-delete:
	minikube image rm $(IMAGE) 2>/dev/null || true

# Load test from local machine (requires port-forward)
loadtest:
	@which hey >/dev/null || (echo "Installing hey..." && go install github.com/rakyll/hey@latest)
	kubectl port-forward svc/url-shortener-svc 8080:80 >/dev/null 2>&1 & \
	sleep 2 && \
	hey -z 60s -c 10 -disable-redirects http://localhost:8080/abc123 && \
	pkill -f "port-forward.*url-shortener-svc"

# Load test running inside cluster (kubectl exec)
loadtest-internal:
	cd loadtest/internal-loadtest && go build -o internal-loadtest .
	kubectl cp loadtest/internal-loadtest/internal-loadtest $$(kubectl get pods -l app=url-shortener-pod -o jsonpath='{.items[0].metadata.name}'):/tmp/
	kubectl exec $$(kubectl get pods -l app=url-shortener-pod -o jsonpath='{.items[0].metadata.name}') -- /tmp/internal-loadtest -requests=1000 -concurrency=10

# Rolling update test (requires hey)
rolling-test:
	@which hey >/dev/null || (echo "Installing hey..." && go install github.com/rakyll/hey@latest)
	./loadtest/rolling-test.sh

clean: undeploy
	docker rmi $(IMAGE) 2>/dev/null || true

