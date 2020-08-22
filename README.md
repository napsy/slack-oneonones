This is a Slack bot that will notify your team about the 1:1 meeting you have
scheduled with them in the next 30 minutes. It is good to remind them of the
agenda and the shared Google Docs meeting-minutes document.

## Prerequisite

1. Slack and the ability to create new bot tokens
2. Google Calendar
3. your scheduled 1:1 meetings must the ``1:1`` in the title and a link to
   shared google docs page for the 1:1 in the description.

## Setup

1. create a new Slack token ID
2. edit ``run.sh`` and copy/paste the token ID
3. add your company email to the ``run.sh`` script
4. run the ``run.sh`` in background.
