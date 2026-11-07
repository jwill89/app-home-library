import shared, { vueConventions, tsStylistic } from '@jwill89/lint-web'

// The shared front-end rules (shared-tooling's lint/web): the Vue style guide in full,
// typescript-eslint's strict and stylistic type-aware rules, accessibility in the
// templates, and the conventions every front end holds. Below them, what only this app
// needs. A rule is turned off here with its reason, or at the one line it does not fit
// with the reason beside it.
export default [
  {
    ignores: [
      'dist/**',
      'coverage/**',
      'test-results/**',
      // Generated from the API document: its shape is openapi-typescript's.
      'src/api/schema.d.ts',
      // Tooling run by node, in no tsconfig: the type-aware rules can't resolve them.
      '*.config.js',
      '*.config.mjs',
      'scripts/**',
      'public/**',
    ],
  },
  ...shared,
  vueConventions,
  tsStylistic,
  {
    name: 'home-library/a11y',
    rules: {
      // A label names its control by holding it or by `for`, and PrimeVue's inputs are
      // controls.
      'vuejs-accessibility/label-has-for': [
        'error',
        {
          required: { some: ['nesting', 'id'] },
          controlComponents: [
            'InputText',
            'InputNumber',
            'Select',
            'MultiSelect',
            'Textarea',
            'ToggleSwitch',
            'Checkbox',
            'RadioButton',
            'DatePicker',
            'SelectButton',
            'Slider',
            'ColorPicker',
            'AutoComplete',
            'InputChips',
          ],
        },
      ],
      // FormField labels the control in its slot, as a <label for> on the control's id.
      'vuejs-accessibility/form-control-has-label': ['error', { labelComponents: ['FormField'] }],
      // A dialog puts the focus on its first control, as the WAI-ARIA dialog pattern asks;
      // that is a component's autofocus, and a bare element's is still refused.
      'vuejs-accessibility/no-autofocus': ['error', { ignoreNonDOM: true }],
    },
  },
]
