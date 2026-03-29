<script lang="ts">
    let {
        items,
        onNavigate,
    }: {
        items: { name: string; url: string; icon: string; href?: string }[];
        onNavigate: (url: string) => void;
    } = $props();

    function onDirClick(e: MouseEvent, url: string): void {
        if (e.metaKey || e.ctrlKey) return;
        e.preventDefault();
        onNavigate(url);
    }
</script>

<ul class="dir__list">
    {#each items as item (item.name)}
        <li class="dir__row">
            <i class="dir__icon material-icons">{item.icon}</i>
            <a
                class="dir__link"
                href={item.href ?? "#"}
                data-url={item.url}
                onclick={(e: MouseEvent) => onDirClick(e, item.url)}
            >
                {item.name}
            </a>
        </li>
    {/each}
</ul>
