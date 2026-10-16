package shell

import (
	"io"
	"sync"
)

// pipes is the stream plumbing shared by sessions that are not backed by a
// file descriptor: the handler writes keystrokes into in and reads output
// from out. Embedding it provides the Reader and Writer methods of Session.
type pipes struct {
	inR  *io.PipeReader
	inW  *io.PipeWriter
	outR *io.PipeReader
	outW *io.PipeWriter

	closeOnce sync.Once
}

func newPipes() *pipes {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	return &pipes{inR: inR, inW: inW, outR: outR, outW: outW}
}

// Reader returns the stream to read shell output from
func (p *pipes) Reader() io.Reader {
	return p.outR
}

// Writer returns the stream to write shell input to
func (p *pipes) Writer() io.Writer {
	return p.inW
}

// closePipes unblocks every reader and writer; safe to call more than once.
// io.Pipe's Close methods never return an error.
func (p *pipes) closePipes() {
	p.closeOnce.Do(func() {
		_ = p.inW.Close()
		_ = p.inR.Close()
		_ = p.outW.Close()
		_ = p.outR.Close()
	})
}
