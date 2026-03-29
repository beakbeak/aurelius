<script lang="ts">
    import { onMount } from "svelte";

    let passwordError = $state(false);

    onMount(() => {
        const query = window.location.search;
        const form = document.getElementById("login-form") as HTMLFormElement;
        form.action += query;

        if (query.match(/^\?failed/)) {
            passwordError = true;
        }
    });
</script>

<main class="login__content modal">
    <form id="login-form" class="login__form ui dialog" action="/login" method="POST">
        <div class="ui__entry-group">
            <label class="ui__entry-label">
                <i class="material-icons">lock</i>
                <input name="username" value="aurelius" autocomplete="on" style="display: none" />
                <input
                    class="ui__entry-input"
                    class:ui__entry-input--error={passwordError}
                    type="password"
                    name="passphrase"
                    placeholder="Password"
                    autocomplete="on"
                />
            </label>
        </div>
        {#if passwordError}
            <p class="ui__error">Login failed.</p>
        {/if}
        <button class="ui__button" type="submit">Login</button>
    </form>
</main>
