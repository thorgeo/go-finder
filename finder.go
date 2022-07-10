package finder

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/b4b4r07/go-finder/source"
	"github.com/pkg/errors"
)

// CLI is the command having a command-line interface
type CLI interface {
	Run() ([]string, error)
	Read(source.Source)
}

// Item is key-value
type Item struct {
	Key   string
	Value interface{}
}

// Items is the collection of Item
type Items []Item

// NewItems creates Items object
func NewItems() Items {
	return Items{}
}

// Add addes item to Items
func (i *Items) Add(key string, value interface{}) {
	*i = append(*i, Item{
		Key:   key,
		Value: value,
	})
}

// Finder is the interface of a filter command
type Finder interface {
	CLI
	Install(string) error
	Select(interface{}) ([]interface{}, error)
}

// Command represents the command
type Command struct {
	Name   string
	Args   []string
	Path   string
	Items  Items
	Source source.Source
}

// Commands represents the command list
type Commands []Command

// DefaultCommands represents the list of default finder commands optimized for quick usage
var DefaultCommands = Commands{
	// https://github.com/junegunn/fzf
	Command{
		Name: "fzf",
		Args: []string{"--reverse", "--height=50%", "--ansi", "--multi"},
	},
	// https://github.com/jhawthorn/fzy
	Command{Name: "fzy"},
	// https://github.com/peco/peco
	Command{Name: "peco"},
	// https://github.com/mooz/percol
	Command{Name: "percol"},
}

// Lookup lookups the available command
func (c Commands) Lookup() (Command, error) {
	for _, command := range c {
		path, err := exec.LookPath(command.Name)
		if err == nil {
			return Command{
				Name:   command.Name,
				Args:   command.Args,
				Path:   path,
				Source: source.Stdin(),
			}, nil
		}
	}
	return Command{}, errors.New("no available finder command")
}

// Run runs as a command
func (c *Command) Run() ([]string, error) {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}

	var stdout bytes.Buffer
	cmd := exec.Command(shell, "-c", c.Path+" "+strings.Join(c.Args, " "))
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	in, _ := cmd.StdinPipe()
	errCh := make(chan error, 1)
	go func() {
		if err := c.Source(in); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
		in.Close()
	}()

	if err := cmd.Start(); err != nil {
		return []string{}, err
	}

	err := <-errCh
	if err != nil {
		return []string{}, err
	}

	if err := cmd.Wait(); err != nil {
		return []string{}, err
	}

	result := stdout.String()
	return trimLastNewline(strings.Split(string(result), "\n")), nil
}

// Select selects the keys in various map
func (c *Command) Select(args interface{}) ([]interface{}, error) {
	switch items := args.(type) {
	case Items:
		var keys []string
		for _, item := range items {
			keys = append(keys, item.Key)
		}
		if len(keys) == 0 {
			return nil, errors.New("no items")
		}
		c.Read(source.Slice(keys))
		selectedKeys, err := c.Run()
		if err != nil {
			return nil, err
		}
		var values []interface{}
		for _, key := range selectedKeys {
			for _, item := range items {
				if item.Key == key {
					values = append(values, item.Value)
				}
			}
		}
		return values, nil
	case []string:
		if len(items) == 0 {
			return nil, errors.New("no items")
		}
		c.Read(source.Slice(items))
		selectedItems, err := c.Run()
		if err != nil {
			return nil, err
		}
		var values []interface{}
		for _, item := range selectedItems {
			values = append(values, item)
		}
		return values, nil
	default:
		return nil, errors.New("Error")
	}
}

func trimLastNewline(s []string) []string {
	if len(s) == 0 {
		return s
	}
	last := len(s) - 1
	if s[last] == "" {
		return s[:last]
	}
	return s
}

// Install does nothing and is implemented to satisfy Finder interface
// This method should be overwritten by each finder command implementation
func (c *Command) Install(path string) error {
	return nil
}

// Read sets the data sources
func (c *Command) Read(data source.Source) {
	c.Source = data
}

// New creates Finder instance
func New(args ...string) (Finder, error) {
	var (
		command Command
		err     error
	)
	if len(args) == 0 {
		command, err = DefaultCommands.Lookup()
		if err != nil {
			return nil, err
		}
	} else {
		path, err := exec.LookPath(args[0])
		if err != nil {
			return nil, errors.Wrapf(err, "%s: not found", args[0])
		}
		command = Command{
			Name:   args[0],
			Args:   args[1:],
			Path:   path,
			Items:  Items{},
			Source: source.Stdin(),
		}
	}
	switch command.Name {
	case "fzf":
		return Fzf{&command}, nil
	case "fzy":
		return Fzy{&command}, nil
	case "peco":
		return Peco{&command}, nil
	default:
		return &command, nil
	}
}
