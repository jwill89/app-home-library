import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
import pluginVue from 'eslint-plugin-vue'
import pluginVueA11y from 'eslint-plugin-vuejs-accessibility'

// The Vue style guide in full, typescript-eslint's type-aware rules, accessibility in the
// templates, and the project's conventions (CLAUDE.md) where a rule can hold them.
// Prettier does the formatting. A rule is turned off here with its reason, or at the one
// line it does not fit with the reason beside it.
export default defineConfigWithVueTs(
  { files: ['**/*.{ts,mts,tsx,vue}'] },
  // Generated from the API document: its shape is openapi-typescript's.
  { ignores: ['dist/**', 'src/api/schema.d.ts'] },
  pluginVue.configs['flat/recommended'],
  ...pluginVueA11y.configs['flat/recommended'],
  vueTsConfigs.recommendedTypeChecked,
  vueTsConfigs.stylisticTypeChecked,
  {
    rules: {
      // An empty string or false falls back as nothing would, so || stays where it is meant.
      '@typescript-eslint/prefer-nullish-coalescing': [
        'error',
        { ignorePrimitives: { string: true, boolean: true } },
      ],
      // <script setup lang="ts"> everywhere, and no inline styles.
      'vue/block-lang': ['error', { script: { lang: 'ts' } }],
      'vue/component-api-style': ['error', ['script-setup']],
      'vue/no-static-inline-styles': 'error',
      'vue/define-macros-order': 'error',
      'vue/require-typed-ref': 'error',
      'vue/prefer-use-template-ref': 'error',
      'vue/no-required-prop-with-default': 'error',
      'vue/no-undef-components': [
        'error',
        { ignorePatterns: ['^Router(Link|View)$', '^FontAwesomeIcon$'] },
      ],
      'vue/no-unused-refs': 'error',
      'vue/no-useless-v-bind': 'error',
      'vue/no-useless-mustaches': 'error',
      'vue/prefer-true-attribute-shorthand': 'error',
      'vue/html-button-has-type': 'error',
      // A label names its control by holding it or by `for`, and PrimeVue's inputs and the
      // site's own are controls.
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
  skipFormatting,
)
