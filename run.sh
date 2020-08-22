#!/bin/bash

export SLACK_TOKEN=<INSERT YOUR SLACK TOKEN HERE>
while :
do
	./remid -o your_company_email@example.com
	sleep 600 # sleep for 10 minutes
done
