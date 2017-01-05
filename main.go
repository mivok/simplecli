package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
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
		panic(err)
	}
	defer rl.Close()

	L := lua.NewState()
	if err = L.DoFile(luaFile); err != nil {
		panic(err)
	}
	defer L.Close()

	registerLuaFunctions(L)

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
			parts = []string{}
		}

		cmd, args := parts[0], parts[1:]

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
			Fn:      L.GetGlobal("do_" + cmd),
			NRet:    0,
			Protect: true,
		}, lua.LString(cmd), argsTable); err != nil {
			fmt.Println(err.Error())
		}
	}
}

func cliVariable(L *lua.LState) int {
	varname := L.ToString(1)
	value := L.ToString(2)
	if value != "" {
		L.SetGlobal(varname, lua.LString(value))
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
