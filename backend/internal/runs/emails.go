package runs

import (
	"bytes"
	"context"
	"html/template"

	"app/internal/db"
	"app/internal/mail"
	"app/internal/runlog"
)

// alertLogLines is how much of the log a failure email quotes: enough to show the error and
// what led to it, the rest is on the plan's page.
const alertLogLines = 40

// failure is a kind of job that can fail: what the email calls it, and its subject.
type failure struct {
	noun    string
	subject string
}

var (
	backupFailed  = failure{noun: "backup", subject: "Backup failed: "}
	restoreFailed = failure{noun: "restore", subject: "Restore failed: "}
)

// Plain like the account emails: a sentence, the error, the end of the log and a link.
var failureHTML = template.Must(template.New("failure").Parse(`<p>The {{.Noun}} of {{.Plan}} failed.</p>
<p><strong>{{.Error}}</strong></p>
<pre style="font-size: 12px; white-space: pre-wrap">{{.Log}}</pre>
<p><a href="{{.Link}}">Open the plan</a></p>
<p>{{.App}}</p>
`))

type failureContent struct {
	Noun  string
	App   string
	Plan  string
	Error string
	Log   string
	Link  string
}

// alert emails every admin with a verified address that a job failed, with a link to page,
// a path of the admin panel. It runs after the job is recorded; a failed send is logged and
// does not change the job.
func (w *Worker) alert(ctx context.Context, kind failure, plan *db.Plan, page string, jobErr error, log *runlog.Log) {
	recipients, err := w.queries.ListAlertRecipients(ctx)
	if err != nil {
		w.logger.Error("Could not list who to tell about a failed job", "plan", plan.ID, "error", err)
		return
	}
	content := failureContent{
		Noun:  kind.noun,
		App:   w.appName,
		Plan:  plan.Name,
		Error: jobErr.Error(),
		Log:   log.Tail(alertLogLines),
		Link:  w.frontendURL + page,
	}
	var html bytes.Buffer
	if err := failureHTML.Execute(&html, content); err != nil {
		w.logger.Error("Could not render the failure email", "plan", plan.ID, "error", err)
		return
	}
	text := "The " + kind.noun + " of " + content.Plan + " failed.\n\n" + content.Error + "\n\n" + content.Log + "\n\n" + content.Link + "\n\n" + content.App + "\n"
	for _, to := range recipients {
		msg := mail.Message{To: []string{to}, Subject: kind.subject + plan.Name, Text: text, HTML: html.String()}
		if err := w.mail.Send(ctx, msg); err != nil {
			w.logger.Error("Could not send the failure email", "plan", plan.ID, "to", to, "error", err)
		}
	}
}
