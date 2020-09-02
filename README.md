# Deployment

Pear requires the following environmental variables:

- `SLACK_SECRET` from the app settings in Slack
- `SLACK_TOKEN` from the app settings in Slack
- `SLACK_CHANNEL` the slack channel Pear with use
- `DATABASE_URL` the PostgreSQL connection string
- `PORT` the port Pear will listen on
- `DEBUG` true for debug logging(optional)

Pear also requires a publically accessible endpoint for slack to post its webhooks to.
