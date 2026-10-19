Fix a duplicate or a drift risk in this order. First make the types or the compiler reject drift. Failing that, derive one copy from the other, through generation, a shared constant or an import. Only when a named boundary rules out sharing, such as two languages with no code generation, an external spec or a build step that cannot change, add a test that pins the copies together, and name that boundary in the fix.

Every new test states the regression it would catch.
