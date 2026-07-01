<?php

// A controller declared in the GLOBAL namespace (no `namespace` statement). Its
// FQN must be the bare short name "LegacyController" with no leading backslash,
// exercising qualify()'s empty-namespace branch (ADR 0006). One public action.
class LegacyController
{
    public function show()
    {
        return null;
    }
}
