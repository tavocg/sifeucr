# Plan de seguridad, rendimiento y consistencia

Revisión del 14 de noviembre de 2026 sobre `f3976c9`. Este documento propone cambios; no implica que estén implementados. El orden prioriza seguridad, después rendimiento y finalmente simplificación. Las dependencias indicadas evitan modificar dos veces una misma solución.

## Alcance y resultados

Se revisaron autenticación y OTP, autorización, cookies/CSRF/CORS, adaptación web de los handlers, TypeScript/templ, consultas y mantenimiento SQL, caché, límites HTTP, nginx, Docker y CI. Se inspeccionaron también las implementaciones instaladas de JWT, validación de correo y transporte SMTP.

Validaciones realizadas:

- `go test -race ./...` con Go 1.26.11: pasó. SQLite incluido; PostgreSQL/MySQL no se ejecutaron porque sus variables `TEST_*_DSN` no estaban configuradas.
- `govulncheck@v1.9.0 ./...`: sin vulnerabilidades conocidas detectadas.
- `npm audit --json --ignore-scripts`: cero avisos.
- `npm run lint` y `npx tsc --noEmit`: pasaron.
- Contenedor nginx temporal usando la configuración de despliegue y un upstream de diagnóstico: ocho POST consecutivos a `/auth/login` produjeron `204,204,204,429,429,429,429,429`. Ocho PATCH a cada ruta de cambio de correo produjeron ocho 204. Esto prueba la omisión del límite del proxy, no una omisión de las cuotas del backend.
- Diagnóstico temporal con el autenticador real y SQLite: emitir R0, renovarlo a R1, revocar R0 devuelve `nil`; el access token de R1 sigue válido y R1 todavía puede renovarse. No se imprimieron credenciales ni se usaron cuentas reales.

No se efectuó una prueba de penetración del despliegue real ni una carga representativa de producción. Los costes señalados por lectura de código no son cuellos de botella medidos. Un resultado limpio de dependencias no certifica la ausencia de vulnerabilidades lógicas. No se añadieron pruebas permanentes ni se modificó código funcional en esta revisión.

Controles existentes que deben conservarse:

- JWT con HS256 verificado, comprobación de caducidad y consulta de sesión/generación sin caché en cada autenticación. Permisos consultados en SQL, no autorizados por los roles potencialmente antiguos del JWT.
- Refresh aleatorios almacenados como SHA-256; rotación transaccional; OTP como HMAC con secreto independiente y consumo atómico. Cuotas persistentes de emisión/verificación que no se reinician al reemplazar un OTP.
- Consultas parametrizadas, paginación de sesiones acotada e índices de sesión/expiración ya existentes. No se identificó una inyección SQL o XSS explotable en los recorridos revisados.
- Cookies HttpOnly/Secure/SameSite=Strict en producción; API exclusivamente Bearer; CSRF en POST web; GET no rota tokens. El cliente no necesita leer tokens ni trasladarlos a localStorage.
- Escape de templ, ausencia de `innerHTML`/`eval` en el cliente propio, límites de cuerpos/cabeceras y tiempos HTTP, SMTP con TLS y validación de certificado.
- Caché limitada y desactivada por defecto, almacenamiento SQLite privado, migraciones con bloqueo y detección de migraciones MySQL incompletas.

## Orden de aplicación

### 1. Corregir y verificar los límites del proxy

- [x] **Prioridad alta; omisión corregida.** Se incluyó PATCH en `$limit_ip` en `deploy/nginx.conf` y `contrib/nginx.example.conf`, compartiendo presupuesto con POST. Los endpoints de cambio y confirmación de correo ya consumen el límite por IP. GET/HEAD/OPTIONS siguen sin consumir presupuesto. La guía identifica los métodos reales de la API.
- [x] Comprobar la matriz método/ruta contra el catálogo Go, incluyendo `SIFEUCR_ROOT`, escapes de URL y barras normalizadas. Validado en ambos ejemplos mediante contenedores temporales: límites compartidos POST/PATCH, rutas prefijadas, `%63`, barras repetidas y segmentos `..`; GET/HEAD/OPTIONS funcionan incluso con el presupuesto agotado. Cabeceras CF/XFF falsas no evaden el límite en el escenario de ingreso simulado. `go test ./...` pasó. No se cambiaron los métodos de la API.
- [ ] Documentar que todo POST `/login` comparte el presupuesto de emisión: verificar, renovar y salir pueden recibir 429 bajo la misma IP/NAT. Medir esa interferencia antes de ajustar tasas. No clasificar seguridad mediante un campo de formulario o cabecera que el cliente pueda falsificar para eludir el límite; conservar las cuotas persistentes por identidad.
- [ ] Validar en Railway real que el edge sustituye `X-Real-IP`, que solo ese ingreso alcanza nginx y que `CF-Connecting-IP` solo se utiliza con un peer Cloudflare verificado. Ensayar cabeceras falsas por dominio Railway directo, dominio Cloudflare e IPv6. Mantener actualizada la lista de redes CF. No describir este control como «por dispositivo»: es por IP pública y por instancia nginx.

**Archivos:** ambas configuraciones nginx y `docs/reverse-proxy.md`. **Aceptación:** PATCH alcanza 429; las cuotas SQL siguen funcionando entre réplicas; peticiones legítimas GET y preflight no se limitan; falsificar cabeceras desde el ingreso público no cambia de presupuesto. La documentación de Railway identifica `X-Real-IP`, pero eso no sustituye verificar la frontera de confianza del despliegue. [Railway](https://docs.railway.com/networking/public-networking/specs-and-limits), [nginx limit_req](https://nginx.org/en/docs/http/ngx_http_limit_req_module.html).

### 2. Cerrar las ambigüedades de revocación y renovación

- [x] **Revocación tras rotación implementada.** `auth.Authenticator.Revoke` resuelve hashes actuales o consumidos, coordina por cuenta y revoca por sesión estable. La migración `20261114000` conserva únicamente hash, identidad/generación y caducidad; no modifica sesiones existentes. No se pueden reconstruir hashes consumidos antes del despliegue.
- [x] La revocación de un refresh identificable usa el mismo orden de bloqueo cuenta→sesión que la renovación. Los hashes desconocidos/no retenidos siguen siendo un no-op idempotente para conservar el contrato; no se afirma que eso pruebe una revocación. La documentación distingue ambos casos.
- [x] La referencia de consumo se guarda en la misma transacción que consumo y reemplazo, hasta la caducidad original de refresh/access; para refresh legacy sin caducidad se usa un TTL configurado desde el consumo o la caducidad de access, si es posterior. Se limpia por lotes de 100 bajo el plazo del mantenimiento y se borra al eliminar la cuenta. El coste es una fila por rotación hasta su caducidad, a medir en el paso 9.
- [x] Política de replay definida: los hashes consumidos permiten revocar pero nunca renovar; un duplicado devuelve 401 y no revoca automáticamente al sucesor. Se evita cerrar sesiones por reintentos legítimos, conservando la limitación de no contener automáticamente a quien ya tenga el sucesor. Una respuesta de renovación perdida todavía exige login. No se añadió caducidad absoluta ni recuperación de replay.
- [x] Logout distingue errores de `Verify`: los fallos de infraestructura devuelven 503 (504 si vence el contexto de la petición) y conservan cookies; los tokens inválidos/expirados siguen el camino de revocación por refresh. Diagnóstico temporal con y sin refresh comprobó errores de dependencia, credenciales inválidas/expiradas y cierre mediante access válido sin refresh.
- [ ] Considerar una caducidad absoluta opcional basada en `created_at`: actualmente cada rotación extiende el refresh y una sesión activa puede durar indefinidamente. Es una decisión de producto/seguridad, no un fallo de expiración del token actual. Mantener la política actual por defecto hasta acordar el cambio.

**Archivos:** `auth/authenticator.go`, `handlers/browser.go`, interfaces/adaptadores SQL, `docs/sessions.md`, `docs/browser.md`. **Dependencias:** cerrar este contrato antes de los pasos 8 y 10. Si requiere esquema, migraciones nuevas coordinadas para los tres motores; conservar sesiones existentes y prever despliegue sin escritores antiguos.

**Aceptación:** logout contra refresh en ambos órdenes, revocación individual y global, access expirado/ausente, error de BD, credenciales antiguas, respuesta perdida y dos réplicas. Una sesión ajena jamás puede revocarse; no se borra una cookie nueva por un fallo tardío. Ejecutar la suite existente y documentar las reproducciones adicionales necesarias antes de implementar.

**Validación de la retención:** `go generate ./database` y `go test -race ./...` con SQLite/PostgreSQL/MySQL pasaron. Diagnósticos temporales en los tres motores comprobaron revocación tras múltiples rotaciones, rechazo de renovación con el hash antiguo sin revocación automática, rollback de emisión, caducidad, limpieza de más de un lote y cascada al borrar la cuenta. También se comprobó POST `/sifeucr/login` en producción con refresh retenido y sin access: redirige, elimina cookies e invalida ambos tokens sucesores. No se añadieron pruebas permanentes; se actualizó la expectativa de versión en la prueba de migración existente.

### 3. Añadir protección del documento sin romper htmz

- [x] Cabeceras comunes en Go: `X-Content-Type-Options: nosniff`, CSP `frame-ancestors 'self'` y `X-Frame-Options: SAMEORIGIN`. Se aplican también a respuestas estáticas, JSON y de enrutamiento; no dependen del proxy. El iframe de htmz conserva permiso del mismo origen.
- [x] CSP específica para HTML propio (root, login/home, 404 HTML): recursos/scripts del mismo origen, `object-src 'none'`, `base-uri 'none'`, `form-action 'self'`, `frame-src 'self'`, favicon `data:` y estilos inline existentes. Primero se probó en Report-Only y después aplicada; no se agregó CSS ni se permitieron scripts inline/eval.
- [x] Redocly conserva solo las cabeceras comunes de framing/nosniff, sin la política de recursos del HTML propio. Su bundle CDN y scripts inline no requieren relajar login/home.
- [x] Se mantiene `Referrer-Policy: strict-origin` y se comparte con el resto del HTML propio. La guía indica HSTS únicamente en el borde HTTPS y advierte sobre `includeSubDomains`; la configuración del dominio real sigue siendo responsabilidad del despliegue.
- [x] El arranque en `--dev` emite WARN sobre orígenes permisivos, bypass CSRF y OTP visibles a nivel debug incluso con secretos largos. No se cambian los overrides de logging ni se habilita dev en ejemplos de producción.

**Validación:** `go test -race ./...` pasó (SQLite; backends externos sin configurar en esta tanda). Chromium aislado comprobó login/OTP, restauración tras cambio de BD, refresh, modal y logout bajo `/sifeucr`, sin excepciones ni violaciones CSP inesperadas, tanto en Report-Only como aplicada. La política aplicada bloqueó un script inline de diagnóstico y el framing desde otro origen. Se verificaron cabeceras de root, login, 404 y documentación. No se añadieron pruebas permanentes ni se modificó el frontend.

**Aceptación:** un origen externo no puede enmarcar login/home; el iframe del mismo origen sigue funcionando; CSP no rompe modal, fechas, SVG, CSS, documentación, formularios ni raíz prefijada. SameSite reduce exposición a framing entre sitios, pero no reemplaza una política de ancestros. [CSP](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CSP), [frame-ancestors](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/frame-ancestors).

### 4. Reducir exposición de datos y trabajo por entradas inválidas

- [x] Ambos nginx usan access logs JSON con método, URI normalizada sin query, estado y duración; no incluyen referrer, Cookie, Authorization ni cuerpo. Se eliminaron campos de correo en errores de los handlers y se ocultan correo/OTP en logs de emisión fuera de desarrollo. Los errores del decoder JSON ya no se registran en crudo, evitando reflejar nombres de campo arbitrarios de hasta 1 MiB.
- [ ] **Pendiente del despliegue:** revisar permisos/retención y redacción de logs nativos de errores de nginx y de Railway/Cloudflare. Se reprodujo que nginx incluye request/referrer en errores de upstream; `log_format` solo sanea access logs. Mensajes de error de dependencias también pueden incluir detalles del proveedor. La guía explicita este límite; no se deshabilitaron los diagnósticos operativos.
- [x] Se conserva el correo en URL/historial para mantener POST/redirect/GET sin añadir estado o cambiar el flujo. La corrección de access logs reduce exposición rutinaria; no se presenta como eliminación del correo de todos los logs o del historial. Una eliminación completa requiere decidir un mecanismo de estado o cambiar el flujo web.
- [x] El servidor exige seis dígitos ASCII antes de HMAC/SQL del desafío. Los códigos malformados conservan el 401 localizado y consumen la cuota por identidad: después de cinco intentos, el siguiente recibe 429. El correo continúa usando el parser existente con sus límites; no se duplicó.
- [x] Se revisaron límites de credenciales: se mantienen los existentes de cuerpo (1 MiB) y cabeceras (32 KiB), sin imponer una longitud arbitraria adicional a JWT/refresh que rompa clientes. Se conserva el rechazo de campos JSON desconocidos y valores múltiples.

**Validación:** `go test -race ./...` pasó (SQLite; backends externos sin configurar en esta tanda). Diagnóstico temporal comprobó cuotas en ambos endpoints, rechazo de códigos vacíos/cortos/largos/no ASCII, código ASCII con cero inicial por el camino normal, ausencia de transacciones de desafío con formato inválido y logs sin entradas grandes reflejadas. Se verificó redacción en producción con debug explícito y disponibilidad de correo/OTP válidos en desarrollo. En contenedores de ambos nginx se comprobó sintaxis, campos exactos del access log y exclusión de queries/referrers/credenciales, conservando límites y métodos seguros. Un upstream fallido confirmó por separado la limitación del error log nativo.

**Aceptación:** access logs sin queries/credenciales y logs de emisión sin correo/OTP en producción; completar la política pendiente para logs de errores/plataforma antes de afirmar privacidad de todos los logs. Entradas grandes y malformadas se rechazan con coste acotado; API y formulario conservan respuestas documentadas. No añadir CAPTCHA o una cola de correo sin evidencia de necesidad.

### 5. Asegurar artefactos y privilegios del despliegue

- [x] **Integridad de Semantic.** `templates/semantic.go` verifica el CSS contra `semanticSHA256`, fijado junto a la tag `v2.2.0`, antes de publicar archivos. Todas las descargas (CSS/licencias) tienen un límite de 1 MiB, comprobado leyendo como máximo el límite más un byte. Se conserva el release minificado y las licencias desde la misma tag. Para actualizar se debe revisar el release y actualizar explícitamente tag/digest; la huella del filename por sí sola no prueba integridad.

  Validación: `go generate ./templates` descargó y verificó el release real. Un diagnóstico temporal sobre el generador comprobó checksum correcto, límite exacto, rechazo por checksum incorrecto/exceso de tamaño/archivo vacío/error HTTP y conservación de CSS, licencias y manifiesto existentes ante esos fallos. `go test ./...` pasó. No se añadieron pruebas permanentes.
- [x] Actions fijadas por SHA completo; bases, frontend BuildKit, servicios SQL de CI y escáner fijados por digest multiplataforma. Dependabot revisa Actions/bases semanalmente y CI también se ejecuta los lunes. La [guía de build](build-security.md) exige revisión manual semanal de referencias que Dependabot puede no descubrir, sin auto-merge.

  CI audita npm antes de ejecutar scripts de instalación y escanea ambos runtimes con Trivy, sin socket Docker ni credenciales del repositorio dentro del escáner. Bloquea HIGH/CRITICAL, sin ignorar hallazgos sin corrección, y conserva govulncheck, permisos de lectura y publicación condicionada al build. Se actualizan paquetes Alpine del runtime durante el build; los repositorios siguen siendo dinámicos y una capa cacheada requiere reconstrucción para recibir correcciones.

  Validación: SHA/digests resueltos contra repositorios/registros oficiales, ambos targets construidos y `nginx -t` correcto; npm audit sin avisos y actionlint correcto. El primer escaneo encontró tres HIGH en libxslt; tras aplicar actualizaciones Alpine, ambos runtimes pasaron el bloqueo HIGH/CRITICAL sin excepciones. `go test ./...` y la prueba existente de réplicas independientes pasaron con PostgreSQL/MySQL (OTP, cuotas, perfil, refresh, revocación, salud y SIGTERM). No se añadieron pruebas permanentes. El workflow remoto se comprobará al subir los cambios.
- [x] MySQL habilita `MultiStatements` únicamente en una conexión temporal de migración, cerrada antes de abrir el pool normal. El runtime fuerza `false` aunque el DSN pida `true`; con `--auto-migrate=false` no se abre conexión de migración. Se documentan credenciales separadas, grants explícitos de MySQL, metadatos de solo lectura y consideraciones PostgreSQL/SQLite en [privilegios de BD](database-privileges.md). No se cambia el arranque automático por defecto ni se añaden conexiones permanentes.

  Validación: diagnóstico temporal con MySQL real confirmó rechazo de sentencias múltiples en ambos modos de apertura, denegación de DDL/escritura de metadatos y funcionamiento de autenticación, rotación/revocación, cuotas y mantenimiento con los grants limitados. La asignación idempotente de roles requiere INSERT/UPDATE en `account_roles`. El comando real `sifeucr migrate` pasó sobre una base nueva con los grants de migración documentados y sin secretos JWT/OTP/SMTP. `go test -race ./...` pasó con SQLite y MySQL; PostgreSQL no se configuró en esta tanda. No se añadieron pruebas permanentes ni se modificaron migraciones.
- [ ] Verificar TLS del DSN cuando la red de BD lo requiera, con validación del servidor, sin `skip-verify`. Documentar el supuesto de red privada si no usa TLS. Mantener Go sin privilegios y sin directorio de escritura; nginx/tini necesitan sus recursos de runtime. No quitar el entrypoint: actualmente supervisa dos procesos y preserva el orden de apagado.

**Aceptación:** checksum incorrecto no publica assets; instalación reproducible; imagen sin avisos explotables pendientes de revisar; usuario de ejecución no puede hacer DDL y `sifeucr migrate` sigue funcionando. No cambiar permisos reducidos de CI ni el flujo de publicación.

### 6. Establecer una línea base de rendimiento

- [ ] **Antes de optimizaciones estructurales.** Medir con `--dev=false`, secretos de prueba y BD aislada: `/home` con 1/100 sesiones, lectura de cuenta, login con mailer de prueba, renovación, errores y estáticos fríos/calientes. Comparar acceso directo y vía nginx, una y dos réplicas, y concurrencia dentro/fuera del tamaño del pool.
- [ ] Registrar p50/p95/p99, errores/429, consultas por operación, esperas del pool, CPU, asignaciones, bytes y antigüedad de filas vencidas. Separar latencia SMTP y SQL de render/JSON/compresión. Perfilar con herramientas locales; no publicar pprof sin autenticación.
- [ ] Fijar objetivo de latencia/capacidad según recursos reales. No inventar porcentajes de mejora ni aumentar pools a ciegas: `réplicas × máximo de conexiones` debe caber en la BD. SQLite tiene una conexión serializada deliberadamente.

**Resultado requerido:** tabla antes/después por cambio y carga reproducible. Las mejoras sin beneficio medible pueden descartarse si añaden complejidad. Mantener los presupuestos de abuso durante las mediciones, distinguiendo pruebas funcionales de pruebas de capacidad autorizadas.

### 7. Aprovechar los assets con hash y afinar compresión

Cambio solicitado después del plan: Semantic y el bundle JS se incrustan una vez por contexto de render con templ.OnceHandle. Se genera el hash CSP exacto del script sin permitir scripts inline arbitrarios; la migración posterior a Datastar se documenta en el paso 10. Los archivos estáticos y sus URLs siguen disponibles, pero las páginas propias ya no los solicitan. Las actualizaciones de sesiones ahora devuelven solo fragmentos y preservan el documento; Datastar es la excepción explícita al inline y se sirve como módulo externo fijado. La recomendación de caché externa solo se mantiene para consumidores opcionales de esos archivos; no implica volver a externalizar los assets solicitados inline por el usuario.

- [ ] **Oportunidad confirmada por inspección.** `handlers/static.go` usa FileServerFS sin `Cache-Control` propio para CSS/JS fingerprinted. Añadir `public, max-age=31536000, immutable` únicamente a assets existentes e inmutables identificados por el manifiesto/hash, en respuestas válidas. No aplicar a `/login`, `/home`, API, HTML de documentación, directorios ni 404.
- [ ] Mantener HTML autenticado y tokens con `no-store`. No añadir caché compartida de páginas de sesión en nginx/Cloudflare. Comprobar que el despliegue gradual sirve los assets referenciados por HTML de ambas versiones: las imágenes actuales solo incluyen el bundle vigente. Caché immutable no resuelve el primer fetch enviado a una réplica de otra versión; conservar assets anteriores o publicarlos en almacenamiento común si se hacen despliegues mixtos.
- [ ] Medir gzip nivel 5 frente a niveles menores y el coste de recomprimir bundles. Solo si compensa, precomprimir assets durante build y servirlos con negociación correcta (`Vary`, HEAD, rangos, `gzip;q=0`); no duplicar ahora todo el árbol de estáticos entre Go y nginx.
- [ ] Verificar reutilización de conexiones al upstream con la versión efectiva de nginx antes de añadir `upstream keepalive`: los valores por defecto cambiaron en versiones recientes. No presentar su ausencia explícita como cuello de botella probado.

**Dependencias:** paso 5 para la integridad y 6 para medición. **Aceptación:** cambios de contenido cambian URL, cargas repetidas reutilizan caché, errores no quedan cacheados y una página autenticada nunca se sirve a otro usuario. [Caché HTTP](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Caching), [keepalive nginx](https://nginx.org/en/docs/http/ngx_http_upstream_module.html).

### 8. Compartir operaciones tipadas entre API y web si su coste lo justifica

- [ ] **Coste confirmado, impacto por medir.** `internal/browser.Call` serializa payloads, invoca un handler con un ResponseWriter en memoria y vuelve a decodificar JSON. `/home` construye JSON de sesiones y luego lo transforma en filas HTML; logout puede verificar el mismo access dos veces.
- [ ] Tras el paso 2, extraer únicamente operaciones compartidas a funciones/servicios tipados: verificar identidad, listar sesiones, emitir/renovar y revocar. Mantener adaptadores HTTP delgados que traduzcan resultados a JSON o HTML/cookies. No crear un framework de repositorios ni un servicio por cada función trivial.
- [ ] Mantener autorización, cuotas OTP, transacciones y clasificación de errores en un único camino. Reutilizar una identidad verificada dentro de la misma petición; nunca cachearla entre peticiones ni omitir la consulta de revocación.
- [ ] El consumo del OTP/creación de cuenta y la emisión de sesión hoy son dos transacciones: un fallo en la segunda consume el código sin entregar sesión. Evaluar unirlos con una operación transaccional pequeña al extraer el servicio, conservando cuotas fuera del rollback y sin mantener locks mientras se envía SMTP. Es una mejora de recuperación, no una reutilización actual del OTP. No reintentar emisión silenciosamente tras un resultado ambiguo.
- [ ] Eliminar `browser.Call`/`Reply` solo cuando todos sus consumidores hayan migrado. Si el ahorro no compensa el refactor, conservar el adaptador y tratar correctamente los errores de `json.Marshal` que hoy se ignoran.

**Aceptación:** mismos endpoints, métodos, estados, JSON, traducciones y metadatos de dispositivo; cookies solo en la superficie web; comparación de consultas/asignaciones con el paso 6. Este paso concentra la simplificación del adaptador y su optimización; no planificar después otro reemplazo paralelo.

### 9. Afinar SQL y mantenimiento sobre el diseño final de sesiones

- [ ] **Riesgo de capacidad, no saturación observada.** Login/email/refresh eliminan como máximo 1000 filas vencidas por tabla y minuto por instancia; rate limits se drenan en lotes de 100 dentro del plazo compartido de cinco segundos. Una llegada sostenida superior al drenaje puede acumular filas, especialmente repartida entre muchas identidades/IP. Medir backlog/antigüedad, y drenar también las otras tablas por lotes bajo el mismo plazo si hace falta.
- [ ] Usar EXPLAIN con cardinalidad representativa para limpieza y listado paginado. Ya existen índices por expiración y `(sub, created_at, session_id)`; no añadir duplicados. Revisar la selectividad de `access_expires_at` junto con `expires_at` después del diseño del paso 2.
- [ ] En varias réplicas, escalonar el mantenimiento si se observan contenciones; no introducir un líder distribuido sin necesidad. Cualquier retención de hashes consumidos del paso 2 debe entrar en este presupuesto.
- [ ] Medir round trips de `TakeRateLimit` en MySQL y locks por cuenta en refresh. Mantener atomicidad y el orden de bloqueo al optimizar. No eliminar cuotas, separar lectura/escritura sin transacción ni cachear autorización para ganar rendimiento.
- [ ] Mantener perfil caching desactivado salvo evidencia; su invalidación es local y no da consistencia inmediata entre réplicas. No añadir Redis, pooling SMTP ni colas globales sin una necesidad medida. Si SMTP domina, evaluar una cola acotada con semántica explícita de entrega y reintentos, sin resetear desafíos/cuotas.

**Dependencias:** 2, 6 y, si se ejecuta, 8. **Aceptación:** backlog converge, limpieza se cancela dentro del presupuesto, cuotas y revocación conservan consistencia, sin regresión p95 bajo carga. Consultas/migraciones coordinadas en los tres motores y generación SQL solo cuando corresponda.

### 10. Unificar formularios y estados del frontend

Cambio solicitado: sustituir htmz por formularios nativos y Datastar; esta decisión reemplaza las recomendaciones originales sobre el iframe.

- [x] Landing y login usan navegación/formularios nativos; solo el estado de restauración automática carga el pequeño controlador de sesión.
- [x] El dashboard carga Datastar desde el binario, fijado por tag y SHA-256, como módulo externo. CSP usa nonce aleatorio por documento, sin `unsafe-eval`.
- [x] Un único `@submit()` adapta formularios a JSON con validación nativa, errores localizados, busy, timeout y Web Locks para los POST fetch. No expone tokens ni cambia autenticación Bearer de la API; mantiene CSRF y permisos del adaptador.
- [x] Refresh/paginación actualizan solo el fragmento de sesiones. No reconstruyen perfil, formularios, header/footer ni assets. El renderizado de filas se conserva en Go.
- [x] Los modales usan `dialog` y `commandfor`; los enlaces de sesión abren el modal mediante Datastar. No hay iframes ni formularios temporales.
- [x] Renovación automática conserva la coordinación entre pestañas y el fallback sin Web Locks; nunca rota en GET, limpia cookies tras un fallo ni reintenta mutaciones ambiguas. Los formularios nativos de login ya no se interceptan para serializarlos.
- [ ] Alinear `/home` y POST `/login` con IDs del catálogo al abordar las rutas web: siguen siendo rutas raw prefijadas. Mantener la capacidad de eliminar esta superficie sin afectar la API.

**Verificación:** ESLint/TypeScript, pruebas existentes y diagnóstico temporal en Chromium con raíz prefijada: login, cambio de correo/error OTP, refresh parcial, restauración, modal, móvil y CSP. Véase `docs/browser.md` para los contratos JSON/HTML y las limitaciones sin JS.

### 11. Simplificar presentación únicamente donde conserve comportamiento

- [ ] `frontend/dates.ts` y `templates/home/dates.go` duplican tablas de meses/reloj. Evaluar `Intl.DateTimeFormat` con instancias reutilizadas por idioma/opciones para el cliente. Mantener `<time datetime>` y fallback UTC del servidor; sin conocer la zona del usuario, el servidor no puede reemplazar correctamente esta conversión local.
- [ ] Comparar salida en español/inglés, medianoche, cambio de año y horario de verano. Intl puede cambiar puntuación y abreviaturas: si conservar exactamente la presentación exige más código, mantener el formatter actual. No cambiar silenciosamente el formato por reducir líneas.
- [ ] La tabla genera hasta 100 modales y vuelve a renderizar la página completa al refrescar. Medir bytes/nodos/tiempo antes de sustituirlos por un modal compartido o respuestas parciales. Un modal compartido necesita asignar `session_id` y texto de forma segura con `textContent`, limpiar estado al cerrar y conservar accesibilidad; no justificarlo solo por suponer que cien filas son lentas.
- [ ] Mantener SVG y HTML nativos, traducciones y separación login/home. No externalizar recursos sensibles ni añadir una dependencia de frontend para sustituir unas pocas líneas.

**Dependencias:** 6 y 10. **Aceptación:** comparar presentación y comportamiento, además de líneas/bundle/DOM; si la mejora de tamaño añade más estados o regresiones, descartarla.

### 12. Cerrar documentación y validar el conjunto

- [ ] Actualizar `docs/browser.md`, `docs/routes.md`, `docs/sessions.md`, `docs/reverse-proxy.md` y ejemplos conforme a lo realmente implementado. Marcar revisiones históricas como tales: `docs/revision.md` contiene afirmaciones antiguas sobre secretos, correo y proxy que ya no describen el estado actual. Este plan no debe duplicar correcciones ya aplicadas.
- [ ] Ejecutar generación específica, pruebas existentes con `-race`, auditorías y la prueba multiinstancia con PostgreSQL/MySQL. Revalidar en navegador aislado cambio de BD con cookies antiguas, renovación simultánea, error SMTP/SQL, pérdida de conexión, cierre de sesión y actualización de assets. No usar sesiones reales del usuario.
- [ ] Mantener commits por problema: proxy, sesiones, cabeceras, privacidad, build, assets, servicios, SQL y frontend. Cada commit funcional debe incluir su documentación y comprobaciones relacionadas; evitar un commit final que mezcle correcciones de seguridad con cosmética.

No crear nuevas suites permanentes sin la autorización exigida por `AGENTS.md`; estos criterios pueden comprobarse con las pruebas existentes y diagnósticos temporales. Si falta cobertura para sostener un cambio delicado, pedir autorización específica para esa prueba al implementar. No se deben marcar tareas como completadas solo porque las comprobaciones antiguas pasen.

## Decisiones que no deben perderse durante la implementación

- La autenticación por cookies pertenece al adaptador web; la API sigue JSON/Bearer. No habilitar credentialed CORS como solución a problemas del formulario.
- CORS no impide que un atacante envíe peticiones fuera del navegador y no sustituye CSRF ni límites de abuso. `--dev` permisivo es intencional y no debe estar en producción.
- No cachear sesiones, permisos, OTP o revocación; no cambiar seguridad por menos consultas.
- No editar migraciones publicadas ni borrar sesiones de usuarios para simplificar una migración. El diseño de replay/retención debe acordarse antes de cambiar esquema.
- El [RFC 9700, sección 4.14](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.14) sirve como referencia para protección de refresh; esta aplicación no se presenta como servidor OAuth completo. Distinguir rotación, detección de replay y caducidad absoluta.
- Sin CSS adicional, cambios de API ni nuevos endpoints por defecto. Menos líneas es un beneficio secundario, no una razón para quitar verificaciones, fallback o modularidad.

Actualización de presentación: se retiró el frontend TypeScript/ESLint. Datastar se carga en el head con SRI; las expresiones compartidas están en templates/interactions.go. Fechas usan Intl del navegador y los dispositivos se muestran como texto. Esto reemplaza las recomendaciones del paso 10 sobre bundle/ESLint y la duplicación de fechas del paso 11.
