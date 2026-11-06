// The standard CSS rules, in the .vue files' style blocks as well, with Tailwind's own
// at-rules known. Prettier formats; this catches what formatting cannot (an unknown
// property, a rule shadowed by one before it, an @import that browsers would ignore).
// A rule is turned off here with its reason, or at the one line with the reason beside it.
export default {
  extends: ['stylelint-config-standard', 'stylelint-config-recommended-vue'],
  rules: {
    'at-rule-no-unknown': [
      true,
      {
        ignoreAtRules: [
          'theme',
          'utility',
          'custom-variant',
          'variant',
          'apply',
          'reference',
          'source',
          'plugin',
          'config',
        ],
      },
    ],
    // Tailwind imports by name, not by url().
    'import-notation': 'string',
    // Scoped styles name their parts block__element--modifier.
    'selector-class-pattern': [
      '^[a-z][a-z0-9]*(-[a-z0-9]+)*(__[a-z0-9]+(-[a-z0-9]+)*)?(--[a-z0-9]+(-[a-z0-9]+)*)?$',
      { message: 'Name classes in kebab-case, block__element--modifier' },
    ],
    // `--color-*: initial` is how Tailwind clears a namespace of its defaults.
    'custom-property-pattern': [
      '^[a-z][a-z0-9]*(-[a-z0-9]+)*(-\\*)?$',
      { message: 'Name custom properties in kebab-case' },
    ],
    // The theme's tokens sit in groups, a blank line between one group and the next.
    'custom-property-empty-line-before': null,
    // A typeface's name is written as the foundry writes it.
    'value-keyword-case': ['lower', { ignoreProperties: ['/^--font-/'] }],
  },
}
