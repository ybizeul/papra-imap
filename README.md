## Introduction

Papra-imap is a simple email collector for your [papra](https://github.com/papra-hq/papra) instance.

Configure IMAP accounts and a papra API key and papra-imap will start monitoring and send to
papra new documents.

## Configuration

Configuration is done in yaml :

```yaml
papra:
    host: https://mypapra.org/
    api_key: ppapi_abcdef
accounts:
    - name: gmail
      email: yann+documents@gmail.com
      username: myusername
      password: my password
      folder: Documents
      organization_id: myorg
```

### Parameters

| key             | description                                   | default    |
|-----------------|-----------------------------------------------|------------|
| name            | A name for the account                        | -          |
| host            | IMAP server host name                         | -          |
| port            | IMAP server port                              | 993        |
| ssl             | If SSL is required or not                     | true       |
| email           | A recipient that must match on imported emails. If empty, all emails are candidate for import | -          |
| folder          | Path to a folder in IMAP server to monitor    | -          |
| organization_id | Organization to import to                     | -          |