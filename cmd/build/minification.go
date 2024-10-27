package main

import (
	"sync"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

type MinificationWorker struct {
	minifier *minify.M
	buffers  *sync.Pool
}

func NewMinificationWorker() *MinificationWorker {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/javascript", js.Minify)
	// m.AddFunc("application/javascript", js.Minify)

	return &MinificationWorker{
		minifier: m,
		// buffers: &sync.Pool{
		//     New: func() interface{} {
		//         return bytes.NewBuffer(make([]byte, 0, 4096))
		//     },
		// },
	}
}

func (m *MinificationWorker) Bytes(mediatype string, content []byte) ([]byte, error) {
	return m.minifier.Bytes(mediatype, content)
}

// func (m *MinificationWorker) Bytes(mediatype string, content []byte) ([]byte, error) {
//     buf := m.buffers.Get().(*bytes.Buffer)
//     buf.Reset()
//     defer m.buffers.Put(buf)

//     if err := m.minifier.Minify(mediatype, buf, bytes.NewReader(content)); err != nil {
//         return nil, err
//     }

//     result := make([]byte, buf.Len())
//     copy(result, buf.Bytes())
//     return result, nil
// }
