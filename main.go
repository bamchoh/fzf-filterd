package main

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"

	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"

	"github.com/Microsoft/go-winio"
)

// --- ラッパー部分 ----

// Accept() で ラップした net.Conn を返したいのでwinioのListenerもラップする
type localWin32PipeListener struct {
	l net.Listener
}

func (l *localWin32PipeListener) Accept() (net.Conn, error) {
	conn, err := l.l.Accept()
	if err != nil {
		return nil, err
	}
	// Accept()で取得したconnをラップして返す
	return &wrappedConn{conn, bytes.NewBuffer([]byte{})}, nil
}

func (l *localWin32PipeListener) Close() error {
	return l.l.Close()
}

func (l *localWin32PipeListener) Addr() net.Addr {
	return l.l.Addr()
}

// net.Conn をラップした構造体
type wrappedConn struct {
	c       net.Conn
	readbuf *bytes.Buffer
}

func (c *wrappedConn) readHeader(reader *bufio.Reader) (contentLength int, err error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return 0, err
	}
	line = bytes.TrimRight(line, "\r\n")
	if strings.HasPrefix(string(line), "Content-Length:") {
		fmt.Sscanf(string(line), "Content-Length: %d", &contentLength)
	}

	if _, err := reader.ReadBytes('\n'); err != nil {
		return 0, err
	}

	return contentLength, nil
}

func (c *wrappedConn) readBuffer() (err error) {
	reader := bufio.NewReader(c.c)
	contentLength, err := c.readHeader(reader)
	if err != nil {
		return err
	}

	body := make([]byte, contentLength)
	n := 0
	for n < contentLength {
		readBytes, err := reader.Read(body[n:])
		if err != nil {
			return err
		}
		n += readBytes
	}
	c.readbuf = bytes.NewBuffer(body)
	return nil
}

func (c *wrappedConn) Read(b []byte) (n int, err error) {
	if c.readbuf.Len() == 0 {
		c.readBuffer()
	}

	return c.readbuf.Read(b)
}

func (c *wrappedConn) Write(b []byte) (n int, err error) {
	// 標準パッケージでは jsonrpc 2.0 用のバージョンが付加されないので
	// 無理やりつける
	// これがないと、クライアント側で受け取ってくれない
	prefix := []byte(fmt.Sprintf("{\"jsonrpc\":\"2.0\","))
	body := append(prefix, b[1:]...)
	// 実データを書き込む前に常にContent-Lengthを送信する
	header := []byte(fmt.Sprintf("Content-Length: %v\r\n\r\n", len(body)))
	// fmt.Print(string(header))
	// fmt.Print(string(body))
	return c.c.Write(append(header, body...))
}

func (c *wrappedConn) Close() error {
	return c.c.Close()
}

func (c *wrappedConn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *wrappedConn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

func (c *wrappedConn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *wrappedConn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *wrappedConn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

// ---- 本体 ----

var pipename = `\\.\\pipe\winiotestpipe`

var list []string

var commandList []string

type (
	TestRPC struct{}

	SetListArgs struct {
		List []string
	}

	SetListReply struct {
		Result int
	}

	FilterArgs struct {
		Pattern string
	}

	FilterReply struct {
		Results []FilterResult
	}

	FilterResult struct {
		Type  string
		Text  string
		Score int
		Pos   []int
	}
)

func (t *TestRPC) SetList(args *SetListArgs, reply *SetListReply) error {
	list = args.List
	reply.Result = 0
	return nil
}

func (t *TestRPC) SetCommandList(args *SetListArgs, reply *SetListReply) error {
	commandList = args.List
	reply.Result = 0
	return nil
}

func (t *TestRPC) innerFilter(args *FilterArgs, texts []string, typ string) (result []FilterResult, err error) {
	caseSensitive := true
	if strings.ToLower(args.Pattern) == args.Pattern {
		caseSensitive = false
	}
	for _, text := range texts {
		chars := util.ToChars([]byte(text))
		var res algo.Result
		var pos *[]int
		res, pos = algo.FuzzyMatchV2(caseSensitive, false, false, &chars, []rune(args.Pattern), true, nil)
		if res.Score > 0 {
			result = append(result, FilterResult{
				Type:  typ,
				Text:  text,
				Score: res.Score,
				Pos:   *pos,
			})
		}
	}

	slices.SortFunc(result,
		func(a, b FilterResult) int {
			return cmp.Compare(a.Score, b.Score)
		})

	return result, nil
}

func (t *TestRPC) Filter(args *FilterArgs, reply *FilterReply) error {
	result1, err := t.innerFilter(args, list, "list")
	if err != nil {
		return err
	}

	result2, err := t.innerFilter(args, commandList, "command")
	if err != nil {
		return err
	}

	result := []FilterResult{}

	result = append(result, result1...)
	result = append(result, result2...)

	slices.SortFunc(result,
		func(a, b FilterResult) int {
			return cmp.Compare(a.Score, b.Score)
		})

	reply.Results = result
	return nil
}

func init() {
	algo.Init("default")
}

func listen(s *rpc.Server) error {
	l, err := winio.ListenPipe(pipename, nil)
	if err != nil {
		return err
	}
	defer l.Close()

	// Listener をラップする
	ll := &localWin32PipeListener{l}

	conn, err := ll.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	s.ServeCodec(jsonrpc.NewServerCodec(conn))
	return nil
}

func main() {
	log.Println("Starting RPC server...")

	s := rpc.NewServer()
	t := &TestRPC{}
	s.Register(t)

	for {
		err := listen(s)
		if err != nil {
			log.Fatal("listen error:", err)
		}
	}
}
