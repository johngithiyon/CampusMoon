APP_NAME=CampusMoon
MAIN=cmd/server/main.go

run:
	sudo docker start mypostgres || true
	sudo docker start myminio || true
	SMTP_HOST=smtp.gmail.com \
	SMTP_PORT=587 \
	SMTP_USERNAME=johngithiyon4@gmail.com \
	SMTP_PASSWORD=pgfsslmluqamhqvn \
	SMTP_FROM=johngithiyon4@gmail.com \
	PORT=8080 \
	go run $(MAIN)
