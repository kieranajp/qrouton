# Spliced into help.sh at @@WARNING@@, so it reuses that script's colour vars
# ($yellow/$dim/$rst). Never run standalone.
printf '  %sWARNING%s  %sCodex agents.max_depth is under 2. Set it to 3%s\n' "$yellow" "$rst" "$dim" "$rst"
printf '           %sin ~/.codex/config.toml for nested subagents.%s\n\n' "$dim" "$rst"
