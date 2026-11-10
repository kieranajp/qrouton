package core

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// Catches a replay and the live output after it overlapping or leaving a gap
// when pages attach while the stream is being published to.
func TestAttachDuringAFloodReplaysUpToExactlyWhereLiveOutputResumes(t *testing.T) {
	const ringLimit = 4096
	stream := newOutputStream(ringLimit)
	var live []Output
	type attached struct {
		replay Output
		from   *uint64
		next   int
	}
	var mu sync.Mutex
	var attaches []attached

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 3000 {
			stream.publish(bytes.Repeat([]byte{byte(i)}, 1+i%97), func(out Output) { live = append(live, out) })
		}
	}()
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var from *uint64
			for i := range 60 {
				err := stream.attach(from, func(out Output) error {
					mu.Lock()
					attaches = append(attaches, attached{replay: out, from: from, next: len(live)})
					mu.Unlock()
					end := out.Offset + uint64(len(out.Data))
					resume := end - min(end, uint64(i*13))
					from = &resume
					return nil
				})
				if err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()

	var all []byte
	for i, out := range live {
		if out.Offset != uint64(len(all)) {
			t.Fatalf("live output %d starts at %d, want %d", i, out.Offset, len(all))
		}
		all = append(all, out.Data...)
	}
	for _, a := range attaches {
		end := a.replay.Offset + uint64(len(a.replay.Data))
		if !a.replay.Replay {
			t.Fatalf("an attach delivered %+v, not marked replay", a.replay)
		}
		if a.next < len(live) && live[a.next].Offset != end {
			t.Fatalf("replay ends at %d, the next live output starts at %d", end, live[a.next].Offset)
		}
		if a.next == len(live) && end != uint64(len(all)) {
			t.Fatalf("a last replay ends at %d, the stream at %d", end, len(all))
		}
		if !bytes.Equal(a.replay.Data, all[a.replay.Offset:end]) {
			t.Fatalf("replay from %d carried other bytes than were written there", a.replay.Offset)
		}
		if a.from != nil && !a.replay.Reset && a.replay.Offset != *a.from {
			t.Fatalf("a resume from %d replayed from %d without a reset", *a.from, a.replay.Offset)
		}
		if a.from == nil && a.replay.Reset {
			t.Fatal("an attach with nothing to resume from was marked reset")
		}
		if len(a.replay.Data) > ringLimit {
			t.Fatalf("a replay carried %d bytes from a %d-byte ring", len(a.replay.Data), ringLimit)
		}
	}
}

// flooded is one kind of terminal flooding output, attachable the way a page is.
type flooded struct {
	attach func(size *Size, from *uint64, deliver func(Output)) error
	live   func() []Output
}

type outputLog struct {
	mu  sync.Mutex
	out []Output
}

func (l *outputLog) record(out Output) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = append(l.out, out)
}

func (l *outputLog) snapshot() []Output {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Output(nil), l.out...)
}

// Catches either terminal kind delivering a replay that overlaps or misses the
// live output a real flooding child goes on to produce.
func TestAttachToAFloodingTerminalOfEitherKindContinuesWithoutGapOrDuplicate(t *testing.T) {
	flood := []string{"/bin/sh", "-c", "head -c 1500000 /dev/zero | tr '\\0' x; printf END; exec cat"}
	for name, open := range map[string]func(t *testing.T) flooded{
		"conversation": func(t *testing.T) flooded {
			log := &outputLog{}
			term, state := testTerm(t, func(event string, payload any) {
				if out, ok := payload.(Output); ok && strings.HasPrefix(event, ptyDataName) {
					log.record(out)
				}
			}, t.TempDir(), flood, os.Environ())
			return flooded{
				attach: func(size *Size, from *uint64, deliver func(Output)) error {
					return term.Attach(state.terminal, size, from, deliver)
				},
				live: log.snapshot,
			}
		},
		"tab": func(t *testing.T) flooded {
			log := &outputLog{}
			w := newWindows(emitFunc(func(event string, payload any) {
				if out, ok := payload.(Output); ok && strings.HasPrefix(event, windowDataName) {
					log.record(out)
				}
			}), testRegistry(t, t.TempDir()))
			t.Cleanup(w.stopAll)
			id, err := w.openWindow(w.shown(), workbench.WindowOptions{
				Kind: workbench.KindTerminal, Label: "flood", Cwd: t.TempDir(), Command: flood,
			})
			if err != nil {
				t.Fatal(err)
			}
			return flooded{
				attach: func(size *Size, from *uint64, deliver func(Output)) error {
					return w.Attach(id, size, from, deliver)
				},
				live: log.snapshot,
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			terminal := open(t)
			if err := terminal.attach(&Size{Cols: 80, Rows: 24}, nil, func(Output) {
				t.Error("the attach that started the terminal delivered a replay")
			}); err != nil {
				t.Fatal(err)
			}
			type attached struct {
				replay Output
				next   int
			}
			var attaches []attached
			const total = 1500003
			var from *uint64
			deadline := time.Now().Add(waitCeiling)
			for {
				if time.Now().After(deadline) {
					t.Fatal("timed out attaching until the flood ended")
				}
				var a attached
				delivered := 0
				if err := terminal.attach(nil, from, func(out Output) {
					delivered++
					a = attached{replay: out, next: len(terminal.live())}
				}); err != nil {
					t.Fatal(err)
				}
				if delivered != 1 {
					t.Fatalf("an attach delivered %d times, want once", delivered)
				}
				attaches = append(attaches, a)
				end := a.replay.Offset + uint64(len(a.replay.Data))
				from = &end
				if end == total {
					break
				}
			}

			live := terminal.live()
			if attaches[0].next == len(live) {
				t.Fatal("the flood ended before the first attach, so none landed in it")
			}
			var all []byte
			for i, out := range live {
				if out.Offset != uint64(len(all)) {
					t.Fatalf("live output %d starts at %d, want %d", i, out.Offset, len(all))
				}
				all = append(all, out.Data...)
			}
			for i, a := range attaches {
				end := a.replay.Offset + uint64(len(a.replay.Data))
				if a.next < len(live) && live[a.next].Offset != end {
					t.Fatalf("replay %d ends at %d, the next live output starts at %d", i, end, live[a.next].Offset)
				}
				if !bytes.Equal(a.replay.Data, all[a.replay.Offset:end]) {
					t.Fatalf("replay %d from %d carried other bytes than were written there", i, a.replay.Offset)
				}
				if i > 0 && !a.replay.Reset && a.replay.Offset != attaches[i-1].replay.Offset+uint64(len(attaches[i-1].replay.Data)) {
					t.Fatalf("replay %d resumed from %d, not where the last one ended", i, a.replay.Offset)
				}
			}
		})
	}
}

// Catches a resume from bytes a tab has dropped, or from beyond what it has
// written, being served as though it continued the page's stream.
func TestAttachFromAnOffsetTheTabNoLongerHoldsReplaysItsWholeRingWithReset(t *testing.T) {
	w, _ := testWindows(t)
	id, err := w.openAgentWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "flood", Cwd: t.TempDir(),
		Command: []string{"/bin/sh", "-c", "head -c 300000 /dev/zero | tr '\\0' x; printf END; exec cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the flood to finish", func() bool {
		read, _ := w.readWindow(id, true)
		return strings.HasSuffix(read.Text, "END")
	})
	replay := func(from uint64) Output {
		t.Helper()
		var got Output
		if err := w.Attach(id, nil, &from, func(out Output) { got = out }); err != nil {
			t.Fatal(err)
		}
		return got
	}
	const total = 300003
	for _, from := range []uint64{0, total - WindowScrollback - 1, total + 1} {
		got := replay(from)
		if !got.Replay || !got.Reset || got.Offset != total-WindowScrollback || len(got.Data) != WindowScrollback {
			t.Fatalf("attach from %d = offset %d, %d bytes, reset %v; want the whole ring with reset", from, got.Offset, len(got.Data), got.Reset)
		}
	}
	if got := replay(total - 3); got.Reset || got.Offset != total-3 || string(got.Data) != "END" {
		t.Fatalf("attach from inside the ring = offset %d, %q, reset %v", got.Offset, got.Data, got.Reset)
	}
}

// Catches an attach that only wants the replay resizing the PTY to whatever the
// page last guessed, or starting a process nobody has sized.
func TestAttachWithoutASizeLeavesThePTYAsItWas(t *testing.T) {
	sized := &Size{Cols: 80, Rows: 24}
	t.Run("conversation", func(t *testing.T) {
		term, state := testTerm(t, func(string, any) {}, t.TempDir(), []string{"/bin/cat"}, os.Environ())
		if err := term.Attach(state.terminal, nil, nil, func(Output) { t.Error("an unstarted conversation replayed") }); err != ErrTerminalNotStarted {
			t.Fatalf("attach without a size before start = %v, want ErrTerminalNotStarted", err)
		}
		if state.process != nil {
			t.Fatal("an attach without a size started the supervisor")
		}
		if err := term.Attach(state.terminal, sized, nil, func(Output) {}); err != nil {
			t.Fatal(err)
		}
		if err := term.Attach(state.terminal, nil, nil, func(Output) {}); err != nil {
			t.Fatal(err)
		}
		if cols, rows, err := ptySize(state.process); err != nil || cols != 80 || rows != 24 {
			t.Fatalf("PTY after a sizeless attach = %dx%d, %v, want 80x24", cols, rows, err)
		}
	})
	t.Run("tab", func(t *testing.T) {
		w, _ := testWindows(t)
		id, err := w.openWindow(w.shown(), workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: "shell", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Attach(id, nil, nil, func(Output) { t.Error("an unstarted tab replayed") }); err != ErrTerminalNotStarted {
			t.Fatalf("attach without a size before start = %v, want ErrTerminalNotStarted", err)
		}
		if err := w.Attach(id, sized, nil, func(Output) {}); err != nil {
			t.Fatal(err)
		}
		if err := w.Attach(id, nil, nil, func(Output) {}); err != nil {
			t.Fatal(err)
		}
		window, _ := w.window(id)
		terminal, _ := window.terminal()
		if cols, rows, err := ptySize(terminal.process); err != nil || cols != 80 || rows != 24 {
			t.Fatalf("PTY after a sizeless attach = %dx%d, %v, want 80x24", cols, rows, err)
		}
	})
}

// Catches a reloaded page drawing a blank conversation: a second attach used to
// be a no-op, so nothing the agent had already said reached it.
func TestASecondAttachOnALiveConversationReplaysItsEarlierOutput(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "printf 'said earlier\\n'; exec cat"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the earlier output", func() bool { return strings.Contains(rec.output(), "said earlier") })
	pid := ptyPid(state.process)

	var replays []Output
	if err := term.Attach(state.terminal, &Size{Cols: 100, Rows: 30}, nil, func(out Output) {
		replays = append(replays, out)
	}); err != nil {
		t.Fatal(err)
	}
	if len(replays) != 1 {
		t.Fatalf("the second attach delivered %d times, want once", len(replays))
	}
	got := replays[0]
	if !got.Replay || got.First || got.Reset || got.ExitCode != nil || got.Offset != 0 || !strings.Contains(string(got.Data), "said earlier") {
		t.Fatalf("second attach = %+v, want a replay of the earlier output", got)
	}
	if ptyPid(state.process) != pid {
		t.Fatal("the second attach forked another agent")
	}
	if cols, rows, err := ptySize(state.process); err != nil || cols != 100 || rows != 30 {
		t.Fatalf("PTY after a sized attach = %dx%d, %v, want 100x30", cols, rows, err)
	}
}

// Catches a reloaded page finding a dead conversation still marked running,
// which it would wait on for output that never comes.
func TestAReplayOfAFinishedConversationCarriesItsExitCode(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "printf 'going\\n'; exit 3"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the exit event", func() bool { return rec.saw(ptyExitName + state.terminal) })
	var got Output
	if err := term.Attach(state.terminal, &Size{Cols: 80, Rows: 24}, nil, func(out Output) { got = out }); err != nil {
		t.Fatal(err)
	}
	if !got.Replay || got.ExitCode == nil || *got.ExitCode != 3 || !strings.Contains(string(got.Data), "going") {
		t.Fatalf("replay of a finished conversation = %+v", got)
	}
}
