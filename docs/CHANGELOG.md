# Changelog

## [0.1.3] - 2024-09-02

* It is now possible to select and navigate through tab-completion candidates in pager mode (#65, #66, thanks [@YangchenYe323](https://github.com/YangchenYe323)!)
* Fixed Home and End keys in certain terminals and multiplexers (#67, #68)
* Fixed crashing edge cases in the Ctrl-T "transpose characters" operation (#69, #70)
* Removed `(Config).ForceUseInteractive`; instead `(Config).FuncIsTerminal` can be set to `func() bool { return true }`

## [0.1.2] - 2024-07-04

* Fixed skipping between words with Alt+{Left,Right} and Alt+{b,f} (#59, #63)
* Fixed `FuncFilterInputRune` support (#61, thanks [@sohomdatta1](https://github.com/sohomdatta1)!)

## [0.1.1] - 2024-05-06

* Fixed zos support (#55)
* Added support for the Home and End keys (#53)
* Removed some internal enums related to Vim mode from the public API (#57)

## [0.1.0] - 2024-01-14

* Added optional undo support with Ctrl+_ ; this must be enabled manually by setting `(Config).Undo` to `true`
* Removed `PrefixCompleterInterface` in favor of the concrete type `*PrefixCompleter` (most client code that explicitly uses `PrefixCompleterInterface` can simply substitute `*PrefixCompleter`)
* Fixed a Windows-specific bug where backspace from the screen edge erased an extra line from the screen (#35)
* Removed `(PrefixCompleter).Dynamic`, which was redundant with `(PrefixCompleter).Callback`
* Removed `SegmentCompleter` and related APIs (users can still define their own `AutoCompleter` implementations, including by vendoring `SegmentCompleter`)
* Removed `(Config).UniqueEditLine`
* Removed public `Do` and `Print` functions
* Fixed a case where the search menu remained visible after exiting search mode (#38, #40)
* Fixed a data race on async writes in complete mode (#30)

## [0.0.6] - 2023-11-06

* Added `(*Instance).ClearScreen` (#36, #37)
* Removed `(*Instance).Clean` (#37)

## [0.0.5] -- 2023-06-02

No public API changes.

## [v0.0.4] -- 2023-06-02

* Fixed panic on Ctrl-S followed by Ctrl-C (#32)
* Fixed data races around history search (#29)
* Added `(*Instance).ReadLine` as the preferred name (`Readline` is still accepted as an alias) (#29)
* `Listener` and `Painter` are now function types instead of interfaces (#29)
* Cleanups and renames for some relatively obscure APIs (#28, #29)

## [v0.0.3] -- 2023-04-17

* Added `(*Instance).SetDefault` to replace `FillStdin` and `WriteStdin` (#24)
* Fixed Delete key on an empty line causing the prompt to exit (#14)
* Fixed double draw of prompt on `ReadlineWithDefault` (#24)
* Hide `Operation`, `Terminal`, `RuneBuffer`, and others from the public API (#18)

## [v0.0.2] -- 2023-03-27

* Fixed overwriting existing text on the same line as the prompt (d9af5677814a)
* Fixed wide character handling, including emoji (d9af5677814a)
* Fixed numerous UI race conditions (62ab2cfd1794, 3bfb569368b4, 4d842a2fe366)
* Added a pager for completion candidates (76ae9696abd5)
* Removed ANSI translation layer on Windows, instead enabling native ANSI support; this fixes a crash (#2)
* Fixed Ctrl-Z suspend and resume (#17)
* Fixed handling of Shift-Tab (#16)
* Fixed word deletion at the beginning of the line deleting the entire line (#11)
* Fixed a nil dereference from `SetConfig` (#3)
* Added zos support (#10)
* Cleanups and renames for many relatively obscure APIs (#3, #9)

## [v0.0.1]

v0.0.1 is the upstream repository [chzyer/readline](https://github.com/chzyer/readline/)'s final public release [v1.5.1](https://github.com/chzyer/readline/releases/tag/v1.5.1).
