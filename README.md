# prulights

What color is the Pru tonight?

prulights showcases the lights atop the Prudential Tower in Boston.

- Go server scrapes the schedule and serves it as a public API.
- Next.js app shows tonight's colors, upcoming schedule, and lets users sign in to track what they've seen.

## demo

<https://prulights.evanslack.dev>

## api

The API is open and needs no key. Docs are at <https://prulights.evanslack.dev/docs>.

```bash
curl https://prulights.evanslack.dev/api/v1/tonight
```

There is also a calendar feed at `/api/v1/calendar.ics`.

## getting started

Run the prebuilt container. The schedule is kept in sqlite under `/data`:

```bash
docker run -p 8080:8080 -v prulights:/data evanofslack/prulights:latest
```

## development

Needs go, node and [just](https://github.com/casey/just).

```bash
just start
```

Then open <http://localhost:3000>. `just check` runs the same checks as CI, and `just` lists the rest.

## credit

The schedule comes from the Prudential Center's
[tower lighting pages](https://www.prudentialcenter.com/experiences/events/31-nights-of-light/).
Not affiliated with the Prudential Center, BXP or Prudential Financial.
