# Makefile for url-shortener

IMAGE := url-shortener:latest

.PHONY: build deploy undeploy logs status port-forward test restart clean minikube-load minikube-delete

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

clean: undeploy
	docker rmi $(IMAGE) 2>/dev/null || true

