package relui

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"golang.org/x/build/internal/relui/db"
	"golang.org/x/build/internal/task"
	"golang.org/x/build/internal/workflow"
)

func TestAwaitFunc(t *testing.T) {
	cases := []struct {
		desc       string
		want       map[string]interface{}
		wantErr    bool
		wantCancel bool
	}{
		{
			desc: "success",
			want: map[string]interface{}{"await": true},
		},
		{
			desc:    "error",
			wantErr: true,
		},
		{
			desc:       "cancel",
			wantCancel: true,
			wantErr:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			didWork := make(chan struct{}, 2)
			success := make(chan interface{})
			done := make(chan interface{})
			wd := workflow.New()

			awaitFunc := func(ctx *workflow.TaskContext) error {
				_, err := task.AwaitCondition(ctx, 10*time.Millisecond, func() (int, bool, error) {
					select {
					case <-success:
						if c.wantCancel {
							cancel()
							return 0, false, ctx.Err()
						} else if c.wantErr {
							return 0, false, errors.New("someError")
						}
						return 0, true, nil
					case <-ctx.Done():
						return 0, false, ctx.Err()
					case didWork <- struct{}{}:
						return 0, false, nil
					}
				})
				return err
			}
			await := workflow.Action0(wd, "AwaitFunc", awaitFunc)
			truth := workflow.Task0(wd, "truth", func(_ context.Context) (bool, error) { return true, nil }, workflow.After(await))
			workflow.Output(wd, "await", truth)

			w, err := workflow.Start(wd, nil)
			if err != nil {
				t.Fatalf("workflow.Start(%v, %v) = %v, %v, wanted no error", wd, nil, w, err)
			}
			go func() {
				if c.wantErr {
					runToFailure(t, ctx, w, "AwaitFunc", &verboseListener{t, nil})
				} else {
					outputs, err := runWorkflow(t, ctx, w, nil)
					if err != nil {
						t.Errorf("runworkflow() = _, %v", err)
					}
					if diff := cmp.Diff(c.want, outputs); diff != "" {
						t.Errorf("runWorkflow() mismatch (-want +got):\n%s", diff)
					}
				}
				close(done)
			}()

			select {
			case <-time.After(5 * time.Second):
				t.Error("AwaitFunc() never called f, wanted at least one call")
			case <-didWork:
				// AwaitFunc() called f successfully.
			}
			select {
			case <-done:
				t.Errorf("AwaitFunc() finished early, wanted it to still be looping")
			case <-didWork:
				close(success)
			}
			<-done
		})
	}
}

func TestCheckTaskApproved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hourAgo := time.Now().Add(-1 * time.Hour)
	p := testDB(ctx, t)
	q := db.New(p)

	wf := db.CreateWorkflowParams{
		ID:        uuid.New(),
		Params:    nullString(`{"farewell": "bye", "greeting": "hello"}`),
		Name:      nullString(`echo`),
		CreatedAt: hourAgo,
		UpdatedAt: hourAgo,
	}
	if _, err := q.CreateWorkflow(ctx, wf); err != nil {
		t.Fatalf("CreateWorkflow(_, %v) = _, %v, wanted no error", wf, err)
	}
	gtg := db.CreateTaskParams{
		WorkflowID: wf.ID,
		Name:       "approve please",
		Finished:   true,
		Error:      nullString("internal explosion"),
		CreatedAt:  hourAgo,
		UpdatedAt:  hourAgo,
	}
	if _, err := q.CreateTask(ctx, gtg); err != nil {
		t.Fatalf("CreateTask(_, %v) = _, %v, wanted no error", gtg, err)
	}
	tctx := &workflow.TaskContext{Context: ctx, WorkflowID: wf.ID, TaskName: gtg.Name}

	got, err := checkTaskApproved(tctx, p)
	if err != nil || got {
		t.Errorf("checkTaskApproved(_, %v, %q) = %t, %v wanted %t, %v", p, gtg.Name, got, err, false, nil)
	}
	tp := db.TaskParams{WorkflowID: wf.ID, Name: gtg.Name}
	task, err := q.Task(ctx, tp)
	if err != nil {
		t.Fatalf("q.Task(_, %v) = %v, %v, wanted no error", tp, task, err)
	}
	if !task.ReadyForApproval {
		t.Errorf("task.ReadyForApproval = %v, wanted %v", task.ReadyForApproval, true)
	}

	atp := db.ApproveTaskParams{
		WorkflowID: wf.ID,
		Name:       gtg.Name,
		ApprovedAt: sql.NullTime{Time: time.Now(), Valid: true},
	}
	_, err = q.ApproveTask(ctx, atp)
	if err != nil {
		t.Errorf("q.ApproveTask(_, %v) = _, %v, wanted no error", atp, err)
	}

	got, err = checkTaskApproved(tctx, p)
	if err != nil || !got {
		t.Errorf("checkTaskApproved(_, %v, %q) = %t, %v wanted %t, %v", p, gtg.Name, got, err, true, nil)
	}
}

func runWorkflow(t *testing.T, ctx context.Context, w *workflow.Workflow, listener workflow.Listener) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	t.Helper()
	if listener == nil {
		listener = &verboseListener{t, nil}
	}
	return w.Run(ctx, listener)
}
