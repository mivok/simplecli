# SimpleCLI - make interactive command line wrappers quickly

SimpleCLI is a tool to allow you to create interactive command line tools
(think a repl or the ftp command). It's intended mainly for wrapping other
commands such as the aws cli or curl.

SimpleCLI provides you with an interative cli, and you implement functions in
lua to perform the actions on the command. Several helpers are provided for
common functionality.

## Why

A while ago I found myself wanting a simple, interactive command line
interface to S3, with commands like `ls`, `put`, `get`, `cp`, `mv` and so on.
I wasn't able to find one ready made, so I wrote a simple tool to get the
functionality I needed. It ended up being a relatively simple wrapper around
the `aws s3` command line tool with some convenience features for storing the
current directory, bucket, aws profile and so on.

Later on, I found myself wanting a similar tool for route53, and another tool
as a simple api client using curl. When I started writing these I realized
that there was a lot of common boilerplate code between the tools, and set out
to write a generic tool that took care of as much of the boilerplate code as
possible. Simplecli is the result.

An earlier version of this tool used toml files and explicit wrappers around
external commands, but I found that even when implementing the s3 tool and
route53 tool that there were too many edge cases for that simple approach to
work. A general purpose language was required to deal with special situations,
and so I chose lua for implementing the command line tools themselves.

## Installation

Install go, and then run:

```
go get github.com/mivok/simplecli
```

## Quick start

All commands are lua functions beginning with `do_`. So to add a command
`hello` you would create a new lua file `myapp.lua` and add a `do_hello`
function:

```
#!/usr/bin/env simplecli
-- vim: filetype=lua

function do_hello(cmd, args)
  io.write("Hello world!\n")
end
```

Now, if you make the file executable and run it, you will have a command line
interface with the `hello` command implemented:

```
$ chmod +x myapp.lua
$ ./myapp.lua
> hello
Hello world!
```

This isn't very exciting, and the main intent of simplecli was to allow you to
wrap external commands, so let's do that. Edit `myapp.lua` and add a new
function:

```
function do_httpbin(cmd, args)
  os.execute(string.format([[
    curl -X POST httpbin.org/post \
      -H 'Content-Type: application/json' \
      -d '["%s"]'
    ]], table.concat(args, '","'))
end
```

This adds a new command that will run `curl` and post your command line
arguments to httpbin:

```
> httpbin foo bar baz
{
  "args": {},
  "data": "[\"foo\",\"bar\",\"baz\"]",
  "files": {},
  "form": {},
  "headers": {
    "Accept": "*/*",
    "Content-Length": "19",
    "Content-Type": "application/json",
    "Host": "httpbin.org",
    "User-Agent": "curl/7.47.1"
  },
  "json": [
    "foo",
    "bar",
    "baz"
  ],
  "origin": "1.2.3.4",
  "url": "http://httpbin.org/post"
}
```

Wrapping curl in this manner lets you quickly create interactive cli clients
for almost any API quickly and easily.

### Variables

Simplecli provides a few convenience functions for commands that work with
variables. These helpers let you work with lua global variables for
substituting into other commands, and environment variables that are inherited
by any commands you run (for example, `AWS_PROFILE` for the `aws` command).

These helpers are:

* `cli_variable(name, value)`
* `cli_envvar(name, value)`
* `cli_toggle(name)`

If you call these from a function, the named global variable (or environment
variable for `cli_envvar` will be set) and its value printed out. If you don't
provide a value (or it's set to nil), then the value will just be printed out.

The `cli_toggle` function is slightly different - it doesn't take a value, and
will toggle a boolean variable between true and false. This can be useful if
you need your commands to have different behavior (e.g. a dry run mode).

Example:

```
-- You can define the variable to give it a default value
myvar="hello"

function do_myvar(cmd, args)
  cli_variable("myvar", args[1])
end

function do_myenv(cmd, args)
  cli_envvar("SOMEVAR", args[1])
end

function do_dryrun(cmd, args)
  cli_toggle("dryrun")
end
```

and when they are used:

```
> myvar
myvar=hello
> myvar foo
myvar=foo
> myenv
SOMEVAR=
> myenv bar
SOMEVAR=bar
> dryrun
dryrun=true
> dryrun
dryrun=false
```
