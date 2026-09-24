# Web internationalization

The Core Web uses `i18next` and `react-i18next`. English is the fallback language
and the source for TypeScript key inference. Simplified Chinese uses the BCP 47
tag `zh-CN`.

Translations are split by feature namespace. Keep reusable actions in `common`,
shell navigation in `navigation`, connection workflow copy in `connection`, and
page-level copy in `pages`. Add a dedicated namespace when a feature grows beyond
page-level labels; do not grow one application-wide translation object.

When adding or changing copy:

1. Add the English key and the `zh-CN` translation in matching namespace files.
2. Consume the key with `useTranslation(namespace)` in React components.
3. Use interpolation for dynamic values instead of concatenating translated text.
4. Keep API values, identifiers, paths, commands, and user-provided content out of
   translation resources.
5. Run the Web tests. The resource parity test rejects missing keys.

The initial language follows the browser preference (`zh*` selects `zh-CN`) unless
the user has made an explicit choice. Explicit choices are stored in local storage;
the application still works when browser storage is unavailable.
