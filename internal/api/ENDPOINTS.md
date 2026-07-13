# Djinni Endpoints

Based on HAR file analysis, here are the main endpoints used by Djinni:

## Candidate Dashboard & Jobs
- **Jobs Matches / For Me:** `https://djinni.co/jobs/?all-keywords=&any-of-keywords=&exclude-keywords=&title=Python` (or similar filter URLs)
- **Job Details:** `https://djinni.co/jobs/<job-id>-<job-slug>/?ref=for_me` (or similar)
- **Similar Jobs (HTMX):** `https://djinni.co/jobs/<job-id>/similar-jobs/`
- **Apply to Job:** Usually a POST to the job details page or an action URL, followed by a redirect to `https://djinni.co/jobs/<job-id>-<job-slug>/?applied=ok` or similar `?applied=1`.

## Inbox / Messages
- **Inbox Dashboard:** `https://djinni.co/my/inbox/`
- **Unread Messages:** `https://djinni.co/my/inbox/?bucket=unread`
- **WebSocket connection:** `wss://djinni.co/ws/candidate/inbox/`
