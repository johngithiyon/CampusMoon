APP_NAME=CampusMoon
MAIN=cmd/server/main.go

run:
	SMTP_HOST=smtp.gmail.com \
	SMTP_PORT=587 \
	SMTP_USERNAME=johngithiyon4@gmail.com \
	SMTP_PASSWORD=pgfsslmluqamhqvn \
	SMTP_FROM=johngithiyon4@gmail.com \
	PORT=8080 \
	go run $(MAIN)
