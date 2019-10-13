# ccb-webflow-api

Easy to use, [JSON](http://www.json.org/) based API for accessing Vous Church data.

This includes:

* Connect card form responses
* Growth track form responses

Currently, this will be a layer on top of the current DB which is [Church Community Build](https://www.churchcommunitybuilder.com/).
CCB uses an XML based approach which is challenging to develop against.
By wrapping this DB in a JSON based API is becomes easier to user, and allows us the option in the future to switch to a different DB is required.

## Goals

* Easy to use JSON based API.
* Business interface layer to the DB.
* Support integration with a visual programming such as [Autopilot HQ](https://www.autopilothq.com).
** We have a small backend team, and development time is a struggle based on schedules. So utilizing an existing and extensible visual programming solution allows this API to be simpler and more expandable.

## Quickstart

```bash
> make run
go run ./
Now listening on: http://localhost:8080
Application started. Press CMD+C to shut down.
```
