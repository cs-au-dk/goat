package dot

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goccy/go-graphviz"
)

// location of dot executable for converting from .dot to .svg
// it's usually at: /usr/bin/dot
var dotExe string

// dotToImageGraphviz generates a SVG using the 'dot' utility, returning the filepath
func dotToImageGraphviz(outfname string, format string, dot []byte) (string, error) {
	if dotExe == "" {
		dot, err := exec.LookPath("dot")
		if err != nil {
			log.Fatalln("unable to find program 'dot', please install it or check your PATH")
		}
		dotExe = dot
	}

	var basepath string
	if outfname == "" {
		basepath = filepath.Join(os.TempDir(), "go-callvis_export.")
	} else {
		basepath = fmt.Sprintf("%s.", outfname)
	}

	dotpath := basepath + "dot"
	if err := ioutil.WriteFile(dotpath, dot, 0644); err != nil {
		return "", err
	}

	fmt.Printf("Exported dot graph to %s\n", dotpath)

	img := basepath + format
	cmd := exec.Command(dotExe, fmt.Sprintf("-T%s", format), "-o", img)
	cmd.Stdin = bytes.NewReader(dot)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command '%v': %v\n%v", cmd, err, stderr.String())
	}
	return img, nil
}

func DotToImage(outfname string, format string, dot []byte) (string, error) {
	if true {
		return dotToImageGraphviz(outfname, format, dot)
	}

	g := graphviz.New()
	graph, err := graphviz.ParseBytes(dot)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := graph.Close(); err != nil {
			log.Fatal(err)
		}
		g.Close()
	}()
	var img string
	if outfname == "" {
		img = filepath.Join(os.TempDir(), fmt.Sprintf("go-callvis_export.%s", format))
	} else {
		img = fmt.Sprintf("%s.%s", outfname, format)
	}
	if err := g.RenderFilename(graph, graphviz.Format(format), img); err != nil {
		return "", err
	}
	return img, nil
}

const tmplCluster = `{{define "cluster" -}}
	{{printf "subgraph %q {" .}}
		{{.Prefix}}
		{{printf "%s" .Attrs.Lines}}
		{{range .Nodes}}
		{{template "node" .}}
		{{- end}}
		{{range .Clusters}}
		{{template "cluster" .}}
		{{- end}}
	{{println "}" }}
{{- end}}`

const tmplEdge = `{{define "edge" -}}
	{{printf "%q -> %q [ %s ]" .From .To .Attrs}}
{{- end}}`

const tmplNode = `{{define "node" -}}
	{{printf "%q [ %s ]" .ID .Attrs}}
{{- end}}`

const tmplGraph = `digraph GoroutineTopology {
	label="{{.Title}}";
	labeljust="l";
	fontname="Arial";
	fontsize="14";
	rankdir="{{or .Options.rankdir "LR"}}";
	bgcolor="lightgray";
	style="solid";
	penwidth="0.5";
	pad="0.0";
	nodesep="{{.Options.nodesep}}";
	remincross="{{or .Options.remincross "true"}}";

	node [shape="ellipse" style="filled" fillcolor="honeydew" fontname="Verdana" penwidth="1.0" margin="0.05,0.0"];
	edge [minlen="{{.Options.minlen}}"]

	{{- range .Clusters}}
	{{template "cluster" .}}
	{{- end}}

	{{range .Nodes}}
	{{template "node" .}}
	{{- end}}

	{{- range .Edges}}
	{{template "edge" .}}
	{{- end}}
}
`

// ==[ type def/func: DotCluster ]===============================================
type DotCluster struct {
	ID       string
	Clusters map[string]*DotCluster
	Nodes    []*DotNode
	Attrs    DotAttrs
	Prefix   string
}

func NewDotCluster(id string) *DotCluster {
	return &DotCluster{
		ID:       id,
		Clusters: make(map[string]*DotCluster),
		Attrs:    make(DotAttrs),
	}
}

func (c *DotCluster) String() string {
	return fmt.Sprintf("cluster_%s", c.ID)
}

func (c *DotCluster) countNodes() int {
	res := len(c.Nodes)

	for _, cluster := range c.Clusters {
		res += cluster.countNodes()
	}

	return res
}

// ==[ type def/func: DotNode    ]===============================================
type DotNode struct {
	ID    string
	Attrs DotAttrs
}

func (n *DotNode) String() string {
	return n.ID
}

// ==[ type def/func: DotEdge    ]===============================================
type DotEdge struct {
	From  *DotNode
	To    *DotNode
	Attrs DotAttrs
}

// ==[ type def/func: DotAttrs   ]===============================================
type DotAttrs map[string]string

func (p DotAttrs) List() []string {
	l := []string{}
	for k, v := range p {
		l = append(l, fmt.Sprintf("%s=%q;", k, v))
	}
	return l
}

func (p DotAttrs) String() string {
	return strings.Join(p.List(), " ")
}

func (p DotAttrs) Lines() string {
	return strings.Join(p.List(), "\n")
}

// ==[ type def/func: DotGraph   ]===============================================
type DotGraph struct {
	Title    string
	Attrs    DotAttrs
	Clusters []*DotCluster
	Nodes    []*DotNode
	Edges    []*DotEdge
	Options  map[string]string
}

func (g *DotGraph) countNodes() int {
	res := len(g.Nodes)

	for _, cluster := range g.Clusters {
		res += cluster.countNodes()
	}

	return res
}

func (g *DotGraph) WriteDot(w io.Writer) error {
	t := template.New("dot")
	t.Option("missingkey=zero") // Make missing map keys return the zero value of appropriate type
	for _, s := range []string{tmplCluster, tmplNode, tmplEdge, tmplGraph} {
		if _, err := t.Parse(s); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, g); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

func (g *DotGraph) ShowDot() {
	xdot, err := exec.LookPath("xdot")
	if err != nil {
		log.Fatalln("unable to find program 'xdot', please install it or check your PATH")
	}

	f, err := os.CreateTemp("", "Goat.*.dot")
	if err != nil {
		log.Fatalln(err)
	}

	defer os.Remove(f.Name())

	if err := g.WriteDot(f); err != nil {
		f.Close()
		log.Fatalln(err)
	} else if err := f.Close(); err != nil {
		log.Fatalln(err)
	}

	var edgeCount int
	for _, e := range g.Edges {
		if str, ok := e.Attrs["style"]; !(ok && strings.Contains(str, "invis")) {
			edgeCount++
		}
	}

	log.Println("Stored dotgraph at", f.Name())
	log.Printf("Graph has %d nodes and %d edges.\n", g.countNodes(), edgeCount)
	log.Println("Starting xdot...")

	if err := exec.Command(xdot, f.Name()).Run(); err != nil {
		log.Printf("Command finished with error: %v", err)
	}
}
