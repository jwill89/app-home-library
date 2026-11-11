// The shared CSS rules, in the .vue files' style blocks as well, with Tailwind's own
// at-rules known. Prettier formats; this catches what formatting cannot (an unknown
// property, a rule shadowed by one before it, an @import that browsers would ignore). A
// rule is turned off here with its reason, or at the one line with the reason beside it.
export default {
  extends: ['@jwill89/lint-web/stylelint', '@jwill89/lint-web/stylelint/tailwind'],
}
