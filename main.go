package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/google/shlex"
	"github.com/valyala/fasttemplate"
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
		L.Pop(1)
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
			L.Pop(1)
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
				helpText := L.GetGlobal("help_" + args[0])
				if helpText.Type() != lua.LTString {
					fmt.Println("No help for command:", args[0])
					continue
				}
				helpString := strings.TrimSpace(helpText.String())
				helpLines := strings.Split(helpString, "\n")
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

		fn, ok := L.GetGlobal("do_" + cmd).(*lua.LFunction)
		if !ok {
			fmt.Println("Unknown command:", cmd)
			continue
		}

		if fn.Proto.NumParameters == 2 {
			// A function can take a third parameter, which will be a filename
			// for a temporary file. We only want to make it though if the
			// function will use it.
			tmpfile, err := ioutil.TempFile("", "simplecli")
			if err != nil {
				fmt.Println(err)
				continue
			}
			tmpfilename := tmpfile.Name()
			// We don't use the file directly, so close it
			tmpfile.Close()
			if err = L.CallByParam(lua.P{
				Fn:      fn,
				NRet:    0,
				Protect: true,
			}, argsTable, lua.LString(tmpfilename)); err != nil {
				fmt.Println(err.Error())
			}
			os.Remove(tmpfilename)
		} else {
			if err = L.CallByParam(lua.P{
				Fn:      fn,
				NRet:    0,
				Protect: true,
			}, argsTable); err != nil {
				fmt.Println(err.Error())
			}
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
		if strings.HasPrefix(k, "help_") {
			// Skip help text
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

func cliCd(L *lua.LState) int {
	varname := L.ToString(1)
	value := L.ToString(2)

	if len(value) == 0 {
		L.SetGlobal(varname, lua.LString("/"))
	} else if strings.HasPrefix(value, "/") {
		L.SetGlobal(varname, lua.LString(value))
	} else {
		parts := strings.Split(value, "/")
		oldvalue := L.GetGlobal(varname).String()
		if oldvalue == "" {
			L.SetGlobal(varname, lua.LString("/"))
		}
		cwd := strings.Split(oldvalue[1:], "/")
		if cwd[len(cwd)-1] == "" {
			cwd = cwd[:len(cwd)-1]
		}
		if len(cwd) == 1 && cwd[0] == "" {
			cwd = []string{}
		}
		for _, part := range parts {
			if part == "." {
				continue
			} else if part == ".." {
				if len(cwd) > 0 {
					cwd = cwd[:len(cwd)-1]
				}
			} else {
				cwd = append(cwd, part)
			}
		}
		newvalue := "/" + strings.Join(cwd, "/")
		if !strings.HasSuffix(newvalue, "/") {
			newvalue = newvalue + "/"
		}
		L.SetGlobal(varname, lua.LString(newvalue))
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

func cliTemplateFunction(L *lua.LState, funcName string) func(io.Writer, string) (int, error) {
	// Returns a go function that calls a lua function by name with no
	// parameters. Used to implement calling lua functions from template
	// strings.
	return func(buf io.Writer, tag string) (int, error) {
		err := L.CallByParam(lua.P{
			Fn:      L.GetGlobal(funcName),
			NRet:    1,
			Protect: true,
		})
		if err != nil {
			return 0, err
		}
		retval := L.Get(-1).String()
		L.Pop(1)
		return buf.Write([]byte(retval))
	}
}

func cliTemplate(L *lua.LState) int {
	templateString := L.ToString(1)
	vars := map[string]interface{}{}
	// First, make environment variables available in templates
	for _, envstr := range os.Environ() {
		parts := strings.SplitN(envstr, "=", 2)
		vars[parts[0]] = parts[1]
	}
	// Next, make all lua global variables and functions available as
	// template variables
	globals := L.Get(lua.GlobalsIndex).(*lua.LTable)
	globals.ForEach(func(klv lua.LValue, v lua.LValue) {
		k := klv.String()
		if strings.HasPrefix(k, "_") {
			// Skip internal variables
			return
		}
		switch t := v.Type(); t {
		case lua.LTString:
			vars[k] = v.String()
		case lua.LTNumber:
			vars[k] = v.String()
		case lua.LTBool:
			vars[k] = v.String()
		case lua.LTFunction:
			vars[k] = fasttemplate.TagFunc(cliTemplateFunction(L, k))
		case lua.LTTable:
			// Tables are accessible with {{tblname[key]}}
			// e.g. foo[bar] or foo[1]
			tbl := v.(*lua.LTable)
			tbl.ForEach(func(tblk lua.LValue, tblv lua.LValue) {
				vars[k+"["+tblk.String()+"]"] = tblv.String()
			})
		}
	})
	// Add local variables too
	debug, ok := L.GetStack(-1)
	if ok {
		idx := 1
		for {
			k, v := L.GetLocal(debug, idx)
			if k == "" {
				break
			}
			tbl, ok := v.(*lua.LTable)
			if ok {
				// Tables are accessible with tblname[key]
				// e.g. foo[bar] or foo[1] or args[1]
				tbl.ForEach(func(tblk lua.LValue, tblv lua.LValue) {
					vars[k+"["+tblk.String()+"]"] = tblv.String()
				})
			} else {
				vars[k] = v.String()
			}
			idx++
		}
	}

	t, err := fasttemplate.NewTemplate(templateString, "{{", "}}")
	if err != nil {
		fmt.Println(err.Error())
		return 0
	}
	L.Push(lua.LString(t.ExecuteString(vars)))
	return 1
}

func registerLuaFunctions(L *lua.LState) {
	L.SetGlobal("cli_variable", L.NewFunction(cliVariable))
	L.SetGlobal("cli_cd", L.NewFunction(cliCd))
	L.SetGlobal("cli_envvar", L.NewFunction(cliEnvvar))
	L.SetGlobal("cli_toggle", L.NewFunction(cliToggle))
	L.SetGlobal("cli_edit", L.NewFunction(cliEdit))
	L.SetGlobal("t", L.NewFunction(cliTemplate))
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
