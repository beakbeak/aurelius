<script lang="ts">
    import { onMount } from "svelte";

    let passwordError = $state(false);
    let form: HTMLFormElement | undefined = $state(undefined);

    onMount(() => {
        if (!form) {
            return;
        }
        const query = window.location.search;
        form.action += query;

        if (query.match(/^\?failed/)) {
            passwordError = true;
        }
    });
</script>

<main>
    <div class="card bg-base-100 shadow-xl/50 p-4">
        <div class="card-title justify-center">
            <p>aurelius</p>
        </div>
        <form bind:this={form} action="/login" method="POST">
            <div class="card-body">
                <label class="input w-full" class:input-error={passwordError}>
                    <i class="material-icons">lock</i>
                    <input
                        name="username"
                        value="aurelius"
                        autocomplete="on"
                        style="display: none"
                    />
                    <input
                        type="password"
                        name="passphrase"
                        placeholder="Password"
                        autocomplete="on"
                    />
                </label>
                {#if passwordError}
                    <div role="alert" class="alert alert-error alert-soft">
                        <span>Login failed.</span>
                    </div>
                {/if}
            </div>
            <div class="card-actions">
                <button class="btn btn-primary btn-block" type="submit">Login</button>
            </div>
        </form>
    </div>
</main>

<style>
    main {
        display: block;
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -80%);
        z-index: 100;
    }
    main::before {
        content: "";
        background: center center / contain url("/img/aurelius.svgz") no-repeat;
        display: block;
        width: 12rem;
        height: 12rem;
        margin: 1rem auto;
    }
</style>
