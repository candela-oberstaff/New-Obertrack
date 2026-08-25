import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'

// Configuración de ESLint 9 (formato plano). Faltaba el archivo —no el linter, que
// ya estaba instalado— así que `npm run lint` fallaba antes de mirar una sola línea.
//
// Se queda en las reglas recomendadas, sin el modo con información de tipos: ese
// exige un proyecto de TypeScript por regla y multiplica el tiempo de análisis, y aquí
// los tipos ya los comprueba `tsc -b` en cada build. Duplicarlo no encuentra más
// errores, sólo tarda más.
export default tseslint.config(
  { ignores: ['dist', 'coverage', 'node_modules'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      // El proyecto usa `any` en las respuestas de la API y en algunos manejadores de
      // error. Convertirlo en error hoy dejaría el lint en rojo desde el primer día y
      // nadie volvería a mirarlo; como aviso, sigue señalando dónde falta un tipo.
      '@typescript-eslint/no-explicit-any': 'warn',
      // Una variable sin usar suele ser código muerto, pero el prefijo _ es la forma
      // habitual de decir "este parámetro existe por la firma y no lo necesito".
      // ignoreRestSiblings deja pasar el idioma de quitar una clave con desestructurado
      // —const { assignees, ...rest } = data—, que no es una variable olvidada sino la
      // forma estándar de decir "todo menos esto".
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrors: 'none',
          ignoreRestSiblings: true,
        },
      ],
      // Los dos siguientes marcan ESTILO ya existente en el proyecto, no defectos: el
      // ternario usado como sentencia (a ? hacerEsto() : hacerLoOtro()) y el catch
      // vacío a propósito. Van como aviso para que se vean sin dejar el lint en rojo
      // por decisiones que ya están tomadas en cientos de sitios.
      '@typescript-eslint/no-unused-expressions': 'warn',
      'no-empty': 'warn',
    },
  },
  // Constructores de correo: arman un documento HTML dentro de una plantilla de texto
  // y escriben <\/script> con la barra escapada a propósito. Es la forma estándar de
  // que ese cierre no corte el documento al insertarlo, y la regla no puede saberlo:
  // dentro de una cadena, la barra escapada y la normal son lo mismo. El comentario de
  // exclusión no cabe ahí —quedaría dentro del HTML generado—, así que se apaga por
  // archivo.
  {
    files: [
      'src/components/Admin/Tools/EmailBuilder/index.tsx',
      'src/components/Admin/Tools/GestorPlantillas/components/TemplateEditor.tsx',
      'src/pages/Email/EmailBuilder.tsx',
    ],
    rules: { 'no-useless-escape': 'off' },
  },
  // Los archivos de prueba corren en Node y usan los globales de Vitest.
  {
    files: ['**/*.test.{ts,tsx}', '**/setupTests.ts', 'scripts/**/*.mjs'],
    languageOptions: {
      globals: { ...globals.node, ...globals.browser },
    },
  },
)
