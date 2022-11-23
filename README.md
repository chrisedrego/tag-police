# tag-police
Tag Police allows you to find resources which are left untagged in your cloud resources.

- Run using various modes
    - CLI - output on standard output
    - CLI/Daemon with UI with Slack Integration- to generate reports
    - Cronjob to run the application

- Problem Goal:
- As Organization expands there are often at times resources which are created and never audited this results in high spike in the cost usage as well as security breach for auditing and making sure the resources we use has purpose meaning and some details as two whos owns it there needs to be tag policy to be used by organization.
- tag-police ensure to scan various resources across different organization and then provide reports or alerts of resource which dont follow the specific tag policy.