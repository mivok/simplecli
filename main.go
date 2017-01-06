package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/google/shlex"
	"github.com/yuin/gopher-lua"
)

func Run(luaFile string) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer rl.Close()

	L := lua.NewState()
	if err = L.DoFile(luaFile); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer L.Close()

	registerLuaFunctions(L)
	parseCommandLineFlags(L)

	// The banner function lets you print some text when the CLI starts
	bannerfn := L.GetGlobal("banner")
	if bannerfn.Type() == lua.LTFunction {
		if err = L.CallByParam(lua.P{
			Fn:      bannerfn,
			NRet:    1,
			Protect: true,
		}); err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(L.Get(-1).String())
	}

	// Set a prompt function to customize the prompt
	promptfn := L.GetGlobal("prompt")
	for {
		if promptfn.Type() == lua.LTFunction {
			if err = L.CallByParam(lua.P{
				Fn:      promptfn,
				NRet:    1,
				Protect: true,
			}); err != nil {
				fmt.Println(err.Error())
			}
			rl.SetPrompt(L.Get(-1).String())
		}
		line, err := rl.Readline()
		// Deal with ^C and ^D
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts, err := shlex.Split(line)
		if err != nil {
			fmt.Println("Error splitting up command string:", err)
			continue
		}

		cmd, args := parts[0], parts[1:]

		// Help for commands is implemented in the help_foo
		if cmd == "help" {
			if len(args) == 0 {
				printCommands(L)
				continue
			} else {
				fn := L.GetGlobal("help_" + args[0])
				if fn.Type() != lua.LTFunction {
					fmt.Println("No help for command:", args[0])
					continue
				}
				if err = L.CallByParam(lua.P{
					Fn:      fn,
					NRet:    1,
					Protect: true,
				}); err != nil {
					fmt.Println(err.Error())
				}
				helpText := strings.TrimSpace(L.ToString(-1))
				helpLines := strings.Split(helpText, "\n")
				for _, line := range helpLines {
					fmt.Println(strings.TrimSpace(line))
				}
				continue
			}
		}

		// Convert args into a lua table
		argsTable := &lua.LTable{}
		for _, arg := range args {
			argsTable.Append(lua.LString(arg))
		}

		fn := L.GetGlobal("do_" + cmd)
		if fn.Type() != lua.LTFunction {
			fmt.Println("Unknown command:", cmd)
			continue
		}

		if err = L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    0,
			Protect: true,
		}, lua.LString(cmd), argsTable); err != nil {
			fmt.Println(err.Error())
		}
	}
}

func printCommands(L *lua.LState) {
	commands := []string{}
	globals := L.Get(lua.GlobalsIndex).(*lua.LTable)
	globals.ForEach(func(klv lua.LValue, v lua.LValue) {
		k := klv.String()
		if v.Type() == lua.LTFunction && strings.HasPrefix(k, "do_") {
			commands = append(commands, k[3:])
		}
	})
	sort.Strings(commands)
	fmt.Println("Available commands:")
	for _, v := range commands {
		fmt.Println(v)
	}
}

func parseCommandLineFlags(L *lua.LState) {
	// Go through all globals and identify any variables we've configured,
	// making them available as flags
	stringArgs := map[string]*string{}
	numArgs := map[string]*float64{}
	boolArgs := map[string]*bool{}

	globals := L.Get(lua.GlobalsIndex).(*lua.LTable)
	globals.ForEach(func(klv lua.LValue, v lua.LValue) {
		k := klv.String()
		if strings.HasPrefix(k, "_") {
			// Skip internal variables
			return
		}
		switch t := v.Type(); t {
		case lua.LTString:
			stringArgs[k] = flag.String(k, v.String(), "Set "+k)
		case lua.LTNumber:
			num := float64(v.(lua.LNumber))
			numArgs[k] = flag.Float64(k, num, "Set "+k)
		case lua.LTBool:
			boolArgs[k] = flag.Bool(k, lua.LVAsBool(v), "Set "+k)
		}
	})
	flag.Parse()
	for k, v := range stringArgs {
		L.SetGlobal(k, lua.LString(*v))
	}
	for k, v := range numArgs {
		L.SetGlobal(k, lua.LNumber(*v))
	}
	for k, v := range boolArgs {
		L.SetGlobal(k, lua.LBool(*v))
	}
}

func cliVariable(L *lua.LState) int {
	varname := L.ToString(1)
	value := L.ToString(2)
	vartype := L.GetGlobal(varname).Type()
	if value != "" {
		if vartype == lua.LTNumber {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				fmt.Println("You must provide a number for numeric variable",
					varname)
				return 0
			}
			L.SetGlobal(varname, lua.LNumber(f))
		} else {
			L.SetGlobal(varname, lua.LString(value))
		}
	}
	fmt.Printf("%s=%s\n", varname, L.GetGlobal(varname).String())
	return 0 // Number of results
}

func cliEnvvar(L *lua.LState) int {
	varname := L.ToString(1)
	value := L.ToString(2)
	if value != "" {
		os.Setenv(varname, value)
	}
	fmt.Printf("%s=%s\n", varname, os.Getenv(varname))
	return 0 // Number of results
}

func cliToggle(L *lua.LState) int {
	varname := L.ToString(1)
	curr := lua.LVAsBool(L.GetGlobal(varname))
	L.SetGlobal(varname, lua.LBool(!curr))
	fmt.Printf("%s=%s\n", varname, L.GetGlobal(varname).String())
	return 0 // Number of results
}

func cliEdit(L *lua.LState) int {
	filename := L.ToString(1)
	fileinfo, err := os.Stat(filename)
	if err != nil {
		fmt.Println("Error getting tempfile modtime:", err)
		L.Push(lua.LBool(false))
		return 1
	}
	previousModtime := fileinfo.ModTime()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, filename)
	c.Stdout = os.Stdout
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	c.Run()

	fileinfo, err = os.Stat(filename)
	if err != nil {
		fmt.Println("Error getting modtime:", err)
		L.Push(lua.LBool(false))
		return 1
	}

	if fileinfo.ModTime() == previousModtime {
		fmt.Println("File was unchanged")
		L.Push(lua.LBool(false))
		return 1
	}

	L.Push(lua.LBool(true))
	return 1
}

func registerLuaFunctions(L *lua.LState) {
	L.SetGlobal("cli_variable", L.NewFunction(cliVariable))
	L.SetGlobal("cli_envvar", L.NewFunction(cliEnvvar))
	L.SetGlobal("cli_toggle", L.NewFunction(cliToggle))
	L.SetGlobal("cli_edit", L.NewFunction(cliEdit))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s CONFIGFILE [OPTIONS]\n", os.Args[0])
		os.Exit(1)
	}
	luaFile := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)
	Run(luaFile)
}
