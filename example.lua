#!/usr/bin/env simplecli
-- vim: ft=lua

function banner()
    -- This is printed when the app starts, and can be used to explain the
    -- purpose of the cli and print any useful information at startup.
    -- E.g. "S3 Client"
    return "Example CLI"
end

function prompt()
    -- Example of a dynamic prompt
    return os.date("%H:%M:%S") .. "> "
end

function do_helloworld(args)
    -- Simple hello world command, prints out the first argument
    io.write("Hello world: ", cmd, " - ", args[1], "\n")
end

-- Help for a function is done by setting help_commandname
-- Leading whitespace and blank lines at the beginning and end will be stripped
help_helloworld = [[
A simple hello world command

Usage: helloworld ARG
]]

function do_anotherhello()
    -- You don't need to accept any parameters in your function if you don't
    -- use them
    io.write("Another hello world!\n")
end

-- You can set a default value for any global variables
-- If you do, they will also be available to set as flags
myvar = "default_value"

function do_myvar(args)
    -- Get/Set a string variable
    -- These variables are accessible as global variables in lua, and you can
    -- predeclare them to get default values (otherwise the default is nil)
    cli_variable("myvar", args[1])
end

function do_profile(args)
    -- Set/get environment variables
    cli_envvar("AWS_PROFILE", args[1])
end

function do_debug(args)
    -- Boolean globals
    cli_toggle("debug_mode")
end

function do_cmd(args)
    -- Run an external command - just use os.execute

    -- Set a default for the first arg
    args[1] = args[1] or "world"

    -- Note: if you don't set a default arg, it will be nil and there will be
    -- an error if you try to concatenate it. You can do the following do get
    -- around the issue:
    -- os.execute("somecommand " .. tostring(args[1])) -- gives "nil"
    -- os.execute("somecommand " .. (args[1] or "")) -- gives ""

    -- The command is executed by the shell, so you can do pipelines here
    os.execute("echo Hello " .. args[1] .. " | sed s/foo/bar/")
end

-- Your do_ function can take a third parameter. If it does, this
-- parameter will be the name of a temporary file created right before
-- your function is called that you can use to download your file to. The
-- file will exist and be blank at the start of the function, and will be
-- deleted right after.
function do_edit(args, tempfile)
    -- The following is how you do an edit workflow, where a command downloads
    -- a file, you edit it in your text editor and then it's re-uploaded if it
    -- was modified.
    os.execute(string.format("curl -s -o %s httpbin.org/get?foo=hello%%20world",
        tempfile))

    -- Edit the file, return if it was changed, and print a message if it
    -- wasn't.
    modified = cli_edit(tempfile)

    if modified then
        os.execute(string.format([[
            curl -X POST -H 'Content-type: application/json' \
                httpbin.org/post -d @%s
            ]], tempfile))
    end
end

function do_cat(args, tempfile)
    -- Display a downloaded file

    -- This is contrived (you could just not add the -o option to curl), but
    -- it's intended to show the behavior where the command you run downloads
    -- the file to disk (e.g. aws s3 cp)
    os.execute("curl -s -o " .. tempfile .. " https://www.example.com/")
    os.execute("cat " .. tempfile)
    -- Or you can do the following to view it with your pager
    -- os.execute("less " .. tempfile)
end

function do_template(args)
    -- An example of using a template
    io.write(t("Myvar is: {{myvar}}\n"))
    -- You can use functions defined in lua also as template variables
    io.write(t("This calls the banner() function: {{banner}}\n"))
    -- Local variables work too
    local localvar = "Hello world"
    io.write(t("localvar is: {{localvar}}\n"))
    -- And you can refer to command arguments too
    io.write(t("first arg: {{args[1]}}; second arg: {{args[2]}}\n"))
    -- Tables are referenced using foo[bar]
    local some_table = {a = "foo", b = "bar", c = "baz"}
    io.write(t("some_table[a]: {{some_table[a]}}\n"))
end

cwd = "/"
function do_cd(args)
    -- This works like cli_variable, but with special handling of the new
    -- value to deal with relative paths
    cli_cd("cwd", args[1])
end
