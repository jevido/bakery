<script lang="ts">
  // The guild rail, as apps/web/src/lib/GuildRail.svelte draws it: a column of
  // the person's Guilds left of the sidebar. Empty until a Bakery is
  // connected; the connected Bakeries' Guilds fill it later.
</script>

<nav class="flex h-full w-[72px] shrink-0 flex-col items-center border-r border-border bg-background py-3" aria-label="Guilds" data-testid="guild-rail">
  <div class="flex min-h-0 w-full flex-1 flex-col items-center gap-2 overflow-y-auto [scrollbar-width:none]"></div>
</nav>
