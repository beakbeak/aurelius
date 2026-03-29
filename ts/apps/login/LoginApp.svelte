<script lang="ts">
    import { onMount } from "svelte";

    let passwordError = $state(false);
    let form: HTMLFormElement | undefined = $state(undefined);

    onMount(() => {
        if (!form) return;
        const query = window.location.search;
        form.action += query;

        if (query.match(/^\?failed/)) {
            passwordError = true;
        }
    });
</script>

<main class="login__content">
    <form bind:this={form} class="login__form ui" action="/login" method="POST">
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

<style>
    .login__content {
        display: block;
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -80%);
        z-index: 100;
    }
    .login__content::before {
        content: "";
        background: center center / contain url("/img/aurelius.svgz") no-repeat;
        display: block;
        width: 12rem;
        height: 12rem;
        margin: 1rem auto;
    }

    .login__form {
        padding: 0.75rem;
        min-width: 15rem;
        box-shadow: 0px 0px 1.5rem rgba(0, 0, 0, 0.5);
        border-radius: 0.25rem;
    }
    .login__form::before {
        content: "aurelius";
        display: block;
        text-align: center;
        font-style: italic;
    }
</style>
