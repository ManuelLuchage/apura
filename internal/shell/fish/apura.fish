# apura — fish integration
# Load it from ~/.config/fish/config.fish:
#     status is-interactive; and apura init fish | source

set -gx APURA_SHELL fish
set -g __apura_bin @APURA_BIN@

# Wrapper: start/resume/end must change variables of *this* shell, which a
# child process cannot do. The binary prints fish code on stdout (because
# APURA_EMIT_ENV is set) and human messages on stderr; we source the code.
function apura --description 'apura: named, annotated terminal sessions'
    switch "$argv[1]"
        case start resume end
            set -l env_code (APURA_EMIT_ENV=fish $__apura_bin $argv)
            set -l rc $status
            if test -n "$env_code"
                printf '%s\n' $env_code | source
            end
            return $rc
        case '*'
            $__apura_bin $argv
    end
end

# Remember where each command starts (a `cd` changes $PWD before postexec).
function __apura_preexec --on-event fish_preexec
    set -g __apura_cwd $PWD
end

# Record every interactive command while a session is active.
function __apura_postexec --on-event fish_postexec
    set -l rc $status
    set -l dur $CMD_DURATION
    set -q APURA_SESSION; or return
    test -n "$argv[1]"; or return
    set -l cwd $__apura_cwd
    test -n "$cwd"; or set cwd $PWD
    # Background + disown: never slow down the prompt, never print job notices.
    $__apura_bin _record --session $APURA_SESSION --exit $rc --duration $dur \
        --cwd $cwd --shell fish -- $argv[1] &>/dev/null &
    disown 2>/dev/null
end

# Optional prompt segment. Example, in fish_right_prompt:
#     apura_prompt
function apura_prompt --description 'print the active apura session, if any'
    set -q APURA_SESSION_NAME; and printf '⏺ %s ' $APURA_SESSION_NAME
end
