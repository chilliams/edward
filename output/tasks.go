package output

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/theothertomelliott/uilive"
	"github.com/yext/edward/tracker"
)

type Follower struct {
	inProgress *InProgressRenderer
	writer     *uilive.Writer

	mtx sync.Mutex
}

func NewFollower() *Follower {
	f := &Follower{
		inProgress: NewInProgressRenderer(),
	}
	f.Reset()
	return f
}

func (f *Follower) Reset() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.writer != nil {
		panic("Follower not stopped correctly")
	}
	f.writer = uilive.New()
	f.writer.RefreshInterval = time.Hour
	f.writer.Start()
}

func (f *Follower) Handle(update tracker.Task) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.writer == nil {
		return
	}

	state := update.State()
	if state != tracker.TaskStatePending &&
		state != tracker.TaskStateInProgress {
		bp := f.writer.Bypass()
		renderer := NewCompletionRenderer(update)
		renderer.Render(bp)
	}
	var buf = &bytes.Buffer{}
	f.inProgress.Render(buf, update)
	fmt.Fprint(f.writer, buf.String())
	f.writer.Flush()
}

func (f *Follower) Done() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.writer.Stop()
	f.writer = nil
}

type NonLiveFollower struct {
	inProgress *InProgressRenderer
}

func NewNonLiveFollower() *NonLiveFollower {
	f := &NonLiveFollower{
		inProgress: NewInProgressRenderer(),
	}
	return f
}

func (f *NonLiveFollower) Handle(update tracker.Task) {
	state := update.State()
	if state != tracker.TaskStatePending &&
		state != tracker.TaskStateInProgress {
		renderer := NewCompletionRenderer(update)
		renderer.Render(os.Stdout)
	}
}

func (f *NonLiveFollower) Done() {
}
