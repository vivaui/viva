# aieval: can a model write a Viva page from the guide?

A dev tool (stdlib only) that measures how well a coding model writes a Viva page given only the
guide a generated project ships (`cmd/viva/initfiles/AGENTS.md.txt`) and a plain request, the way a
user would prompt it. Use it after changing `flex`, a component or the guide: the failures it finds
are gaps in the framework or the guide, not in the model.

```sh
export DEEPSEEK_API_KEY=...            # see "Keys"
go run ./tools/aieval -only deepseek-flash -runs 3          # every task, three samples each
go run ./tools/aieval -task modal,splitter -runs 5 -png     # some tasks; -png adds a screenshot of each page
go run ./tools/aieval -list                                  # the tasks
```

For every task, model and run it builds the `viva` command from this checkout, makes a project with
`viva init` (so the files are exactly what a user gets), sends the guide, the template `main.go` and the
request to the model, extracts the Go file, runs `go vet` and then `viva vet`, and feeds what they report back for up to
`-repairs` rounds (default 3). A page that compiles is driven headless (`github.com/vivaui/viva/headless`) through the
task's expectations. Per run it records: compiled on the first try, repaired in k rounds, or failed; each
expectation; tokens and wall time. Output (default under `$TMPDIR/viva-aieval/<time>/`): a table on
stdout and in `table.md` (pass rates per task and model, failed expectations, the most common compiler
complaints), `results.json`, and `raw/<model>/<task>/run<n>/` with every reply and the generated project
(keep these out of the repository; they are not scrubbed of the model's text, only of the key).

## Models and keys

Any OpenAI-compatible chat completions endpoint. Configure with `~/.config/viva-aieval.toml`:

```toml
[[model]]
name = "deepseek-flash"
base_url = "https://api.deepseek.com"
model = "deepseek-flash"          # defaults to name
key_env = "DEEPSEEK_API_KEY"      # or key_file = "~/.secrets/deepseek"
max_tokens = 16000
temperature = 0.2                 # negative: not sent
extra = '{"thinking":{"type":"disabled"}}'   # merged into the request body
```

or the environment (`VIVA_AIEVAL_BASE_URL`, `VIVA_AIEVAL_MODEL`, `VIVA_AIEVAL_KEY`, optional
`VIVA_AIEVAL_EXTRA`), or repeatable `-model name=baseurl,keyenv[,extra-json]`. `-only a,b` selects among them.

The key is only ever read at run time from the environment variable or the file, never stored in a
config, a result or a log, and every output is scrubbed of it. There is deliberately no option to read
it from another application's database.

## Tasks

`tasks/*.md`: a front matter between `---` lines, then the request. Directives, in the order they run:

| directive | meaning |
| --- | --- |
| `title:` | name in reports |
| `needs:` | a capability the framework lacks: the task still runs but is listed apart and left out of the totals |
| `source: X` | `main.go` must contain `X` (what a headless check cannot see, such as a second window) |
| `text: X` / `not-text: X` | a painted text contains `X` / none does |
| `hits: N` | at least N interactive regions |
| `click: L`, `click-at: X Y`, `type: T`, `key: K`, `wait: MS` | what a user does (`L` is a control's label, or, when no control has it, the text field showing that placeholder or text; `K` as in `headless.Session.Keys`) |
| `drag: h 80` | press on the region with a horizontal (`v`: vertical) resize cursor and drag it 80 px; `moved:` then checks that text moved; `resizable:` that such a region exists |
| `before: A \| B` | text A is painted above text B (a sort order) |
| `right-of: A \| B` | text A starts at least 20 px further right than text B (message bubbles, alignment) |
| `animates: MS` | the painted shapes change as the animation clock moves MS |
| `bg-mark:` / `bg-changed:` | the window background differs from what it was at the mark (a theme switch) |
| `layout:` | at 640x480 and 1200x800: no text partly outside the window, no two texts or controls overlapping (text extent is estimated: half an em per character) |

The checks are executed by `check_test.go.txt`, which is written into the generated project as a test.
Write a task like a user request, and make a task's expectations true of every reasonable reading of it
(name the sample data and say what the initial state is), otherwise a failure measures the task and not the
guide. Before counting failures of a new task against the guide, make sure the framework can do it
(write the page yourself, `/tmp`-style, and run the expectations on it); if it cannot, mark `needs:`.
