import javascriptRules from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import typescriptRules from "typescript-eslint";

export default typescriptRules.config(
  { ignores: ["dist", "src-tauri"] },
  {
    extends: [javascriptRules.configs.recommended, ...typescriptRules.configs.recommended],
    files: ["**/*.{ts,tsx,js,mjs}"],
    languageOptions: {
      ecmaVersion: 2022,
      globals: { ...globals.browser, ...globals.node },
    },
    plugins: { "react-hooks": reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Project rule: identifiers must be semantic. No "a", "x", "e", "i"...
      "id-length": ["error", { min: 3, exceptions: ["id", "ok", "fs", "os"], properties: "never" }],
    },
  },
);
