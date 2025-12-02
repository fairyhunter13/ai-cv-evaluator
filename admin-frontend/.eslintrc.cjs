/* ESLint configuration for admin-frontend (Vue 3 + TypeScript + Vite)
 * This is used by `npm run lint` which is invoked from `make lint-frontend` / `make lint-all`.
 */

module.exports = {
  root: true,
  env: {
    browser: true,
    es2021: true,
    node: true,
  },
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
  },
  extends: [
    "eslint:recommended",
    "plugin:vue/vue3-recommended",
    "@vue/eslint-config-typescript",
  ],
  rules: {
    // Keep rules relatively relaxed; CI is mainly enforcing syntax and obvious issues.
    // Allow empty catch blocks (used for defensive try/catch around auth init).
    "no-empty": ["error", { allowEmptyCatch: true }],

    // Vue pages are short, single-word names like Dashboard/Jobs; don't enforce multi-word.
    "vue/multi-word-component-names": "off",

    // Jobs.vue uses an intentional v-else-if chain; avoid blocking on duplicate-condition heuristics.
    "vue/no-dupe-v-else-if": "off",

    // Downgrade TS unused vars to warning and ignore underscore-prefixed helpers/args.
    "@typescript-eslint/no-unused-vars": [
      "warn",
      {
        argsIgnorePattern: "^_",
        varsIgnorePattern: "^_",
        caughtErrorsIgnorePattern: "^_",
      },
    ],
  },
  overrides: [
    {
      files: ["tests/**/*.spec.ts"],
      rules: {
        // Playwright specs often keep extra imports/helpers for readability; don't block CI on them.
        "@typescript-eslint/no-unused-vars": "off",
        "no-useless-escape": "off",
        "no-empty": "off",
      },
    },
  ],
};
