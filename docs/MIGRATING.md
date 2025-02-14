# Migrating

ergochat/readline is largely API-compatible with the most commonly used functionality of chzyer/readline. See our [godoc page](https://pkg.go.dev/github.com/cogentcore/readline) for the current state of the public API; if an API you were using has been removed, its replacement may be readily apparent.

Here are some guidelines for APIs that have been removed or changed:

* readline used to expose most of `golang.org/x/term`, e.g. `readline.IsTerminal` and `readline.GetSize`, as part of its public API; these functions are no longer exposed. We recommend importing `golang.org/x/term` itself as a replacement.
* Various APIs that allowed manipulating the instance's configuration directly (e.g. `(*Instance).SetMaskRune`) have been removed. We recommend using `(*Instance).SetConfig` instead.
* The preferred name for `NewEx` is now `NewFromConfig` (`NewEx` is provided as a compatibility alias).
* The preferred name for `(*Instance).Readline` is now `ReadLine` (`Readline` is provided as a compatibility alias).
* `PrefixCompleterInterface` was removed in favor of exposing `PrefixCompleter` as a concrete struct type. In general, references to `PrefixCompleterInterface` can be changed to `*PrefixCompleter`.
* `(Config).ForceUseInteractive` has been removed. Instead, set `(Config).FuncIsTerminal` to `func() bool { return true }`.
