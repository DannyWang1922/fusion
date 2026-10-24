# Full article worker

Install dependencies from the repository root with `npm ci --prefix tools/fulltext`. The computer running Fusion needs Node.js and Google Chrome. The worker uses an isolated, ordinary Chrome window because OpenAI currently rejects requests from headless Chrome. The window closes after extraction and does not use a logged-in profile.

Enable full content in a feed's advanced settings. Opening an article starts a background browser job. You can read the RSS summary while it loads. The extracted body is cached separately in SQLite for reading and translation. Failed attempts show a retry action. Interactive charts are not reproduced as live widgets.

The worker currently accepts public OpenAI article URLs only. It limits browser requests to the publisher and its asset domains. Frontend article sanitization also applies to extracted HTML. See [configuration](../../backend/internal/config/config.go), [URL checks and process lifecycle](../../backend/internal/fulltext/fulltext.go), and [extraction](extract.mjs) for implementation details.

Category filters use exact RSS category tags. Date filters use a rolling window. Existing out-of-scope records are hidden without changing their read state; future feed refreshes only insert matching new articles. Expanding a filter forces an unconditional feed refresh to recover matching entries still present in the publisher's RSS feed.
