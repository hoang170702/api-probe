# API Case Runner user guide

API Case Runner lets a tester take one API request as a `curl` command and run it with many data sets, without writing code. This guide covers what the current version does: **paste a curl, check the request, and set a rule for every field of its JSON body**. Entering test cases, running them and exporting reports come in the next releases ([BRD-001 §18](brd/BRD-001.md#18-implementation-plan-small-steps)).

Vietnamese version: [user-guide.vi.md](user-guide.vi.md).

## 1. Start API Case Runner

API Case Runner is one program that serves both the web page and its API.

```sh
cd caserunner
scripts/build.sh          # once, or after pulling new code
backend/bin/caserunner
```

Then open **http://127.0.0.1:8080** in a browser. The tag at the top right shows **BE OK** and the version when the tool is ready.

| Want to | Do |
|---|---|
| Keep it running after closing the terminal | `nohup backend/bin/caserunner > caserunner.log 2>&1 &` and stop it later with `kill <pid>` |
| Use another port, or let teammates open it | `CASERUNNER_BIND=0.0.0.0:9090 backend/bin/caserunner` (by default only your own machine can reach it) |
| Stop it | `Ctrl+C` in its terminal |

Building needs Go 1.27+ and Node.js 20.19+; if a developer hands you a ready-built `caserunner` binary, just run it.

## 2. Copy the request as curl

| From | How |
|---|---|
| Chrome / Edge DevTools | Network tab → right-click the request → **Copy** → **Copy as cURL (bash)** or **Copy as cURL (cmd)** |
| Firefox DevTools | Network tab → right-click the request → **Copy Value** → **Copy as cURL** |
| Postman | Open the request → **Code** (`</>`) → **cURL** → copy |

Both the bash and the Windows cmd style work; you do not need to clean the text up.

## 3. Paste and analyze

Paste the command into **1. Curl** and click **Analyze** (or press **Ctrl+Enter**).

![The curl panel with a pasted command](images/01-paste-curl.png)

## 4. Check the request

**2. Request** shows what API Case Runner understood:

- **Request**: the method and the URL (the copy icon copies the URL).
- **Body**: `JSON`, `No body`, or `Unsupported`.
- **Warnings**: things the tool ignored or changed, such as *"Ignored --compressed"*. Read them; they never stop you, but they explain any difference from running the curl yourself.
- **Headers**: every header in order. Values of `Authorization`, `Cookie`, `X-Api-Key` and headers containing `token` or `secret` are masked (first 6 characters + `***`). Turn on **Show sensitive values** to see them in full.

![Request summary and field rules after Analyze](images/02-request-and-fields.png)

## 5. Set the field rules

**3. Field rules** lists every field of the JSON body, in the body's order:

| Column | Meaning |
|---|---|
| **Field path** | Where the field sits: `data.account.id` is `id` inside `account` inside `data`; `items[0].sku` is `sku` in the first item of the `items` array. A key that contains a dot or a bracket is written in quotes, e.g. `["key.with.dot"]`. |
| **Type** | `string`, `number`, `boolean`, `null`, or `object` / `array` for an empty `{}` / `[]`. |
| **Sample value** | The value in your curl, written the way you would type it in a test case: `<EMPTY>` is an empty string `""`, `<NULL>` is `null`. Numbers keep every digit, e.g. a 20-digit account number. |
| **Rule** | **Required** (default) or **Unrequired**. |

What the rule will mean when you run cases:

- **Required**: every case must give this field a value. A case that leaves it empty is **BLOCKED** and never sent.
- **Unrequired**: a case may leave it empty, and then the field is **left out of the request entirely**. Use this to test how the API handles a missing field.

**All Required** and **All Unrequired** set every rule at once; the counter above the table shows how many of each you have. If you edit the curl and analyze again, fields that still exist keep the rule you chose.

## 6. When the curl cannot be used

The message under the curl explains what to fix. The position counts characters from the start of the text.

![A curl that reads its body from a file](images/03-parse-error.png)

| Message (start) | What to do |
|---|---|
| *Missing closing ' for the quote opened at position N* | The copy was cut off or a quote is missing; copy the request again. |
| *A curl command must start with curl* | Paste only the command, starting at `curl`. |
| *No URL found* / *Found a second URL* | Keep exactly one URL in the command. |
| *-d @file reads the body from a file* | Replace `@file` with the body itself. |
| *-G is not supported* / *-T is not supported* | These change the request in ways API Case Runner cannot reproduce; remove them and put the data in the body. |
| *Only http and https URLs are supported* | Use an `http://` or `https://` URL. |
| *The JSON body cannot be split into fields: duplicate key* | The body repeats a key in one object; remove the duplicate. |

A request with a form or file-upload body (`-F`, `--data-urlencode`) or a non-JSON body is analyzed, but shows **v1 supports JSON bodies only** and cannot be run:

![A form body is not supported](images/04-unsupported-body.png)

Common warnings that need no action:

| Warning | Meaning |
|---|---|
| *Ignored --compressed*, *Ignored -s*, … | Output and transport options; they do not change the request. |
| *Ignored -L: redirects are never followed* | A redirect (3xx) will be reported as the result instead of being followed. |
| *Ignored --max-time: the run's timeout applies* | Timeouts are set per run in API Case Runner. |
| *No Content-Type header* | The API may not read the body as JSON; add `-H 'Content-Type: application/json'` if it should. |
| *The URL has no scheme; using http://…* | Add `https://` if the API needs it. |

## 7. Good to know

- API Case Runner keeps nothing on disk. Closing or reloading the page loses what you entered.
- Your curl, including tokens, is sent only to your own API Case Runner server and is never written to its log.
- Use API Case Runner against LOCAL, DEV or UAT environments, never production. A host allowlist to enforce this is planned ([BRD-001 §13](brd/BRD-001.md#13-safety--security)).

## 8. Troubleshooting

| Symptom | Fix |
|---|---|
| Red **BE down** tag and *Backend unreachable* banner | The API Case Runner program is not running or was stopped; start it again and reload the page. |
| `scripts/build.sh` stops with *the Windows Node.js is on PATH* or *node was not found*, or prints `UNC paths are not supported` | Under WSL the build needs Node.js installed inside Linux, not the Windows one: install it with nvm (`nvm install 22`) as the script explains, open a new terminal, and build again. |
| The page says *The frontend is not built into this binary* | The program was built without the web page; build it with `scripts/build.sh`. |
| `address already in use` when starting | Another program uses port 8080; start with `CASERUNNER_BIND=127.0.0.1:9090` and open that port. |
| The service stops right away with a message naming a `CASERUNNER_` variable | That environment variable has an invalid value; fix or unset it ([BRD-001 §20](brd/BRD-001.md#20-configuration-env)). |
