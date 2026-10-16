<script>
	import { api } from '$lib/api.js';
	import { labels as sharedLabels } from '$lib/store.js';
	import { showToast } from '$lib/ui.js';

	// Matches the exclusive-group taxonomy in AGENTS.md: these sort first, in
	// this order; any other group the workspace has follows, alphabetically.
	const CANON_ORDER = ['repo', 'platform', 'type', 'domain', 'triage'];
	const SWATCHES = ['#38BDF8', '#A78BFA', '#34D399', '#FBBF24', '#F87171', '#FB923C', '#8A8F9C', '#6B7280'];

	let groups = $state([]);
	let labels = $state([]);
	let loading = $state(true);

	let newGroup = $state('');
	let newName = $state('');
	let newColor = $state(SWATCHES[0]);
	let adding = $state(false);
	let addErr = $state('');

	async function load() {
		loading = true;
		try {
			const [g, l] = await Promise.all([api.labelGroups(), api.labels()]);
			groups = g || [];
			labels = l || [];
		} catch (e) {
			showToast('Could not load labels: ' + (e?.message || e), 'error');
		} finally {
			loading = false;
		}
	}
	load();

	const groupRows = $derived.by(() => {
		const byId = new Map(groups.map((g) => [g.id, g]));
		const byName = new Map();
		for (const l of labels) {
			const g = l.groupId ? byId.get(l.groupId) : null;
			const gname = g?.name || 'ungrouped';
			if (!byName.has(gname)) byName.set(gname, { id: g?.id ?? null, name: gname, exclusive: g?.exclusive ?? false, items: [] });
			byName.get(gname).items.push(l);
		}
		// The ungrouped section always renders: it is a drop target even when
		// no label is ungrouped yet.
		if (!byName.has('ungrouped')) byName.set('ungrouped', { id: null, name: 'ungrouped', exclusive: false, items: [] });
		// Include a known group even with zero labels yet, so "+ Add label" is
		// reachable for it.
		for (const g of groups) if (!byName.has(g.name)) byName.set(g.name, { id: g.id, name: g.name, exclusive: g.exclusive, items: [] });
		const rows = [...byName.values()];
		rows.sort((a, b) => {
			const ai = CANON_ORDER.indexOf(a.name);
			const bi = CANON_ORDER.indexOf(b.name);
			if (ai !== -1 || bi !== -1) return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
			return a.name.localeCompare(b.name);
		});
		return rows;
	});

	const groupNames = $derived([...new Set([...CANON_ORDER, ...groups.map((g) => g.name)])]);

	function startAdd(groupName) {
		newGroup = groupName;
		document.getElementById('label-name-input')?.focus();
	}

	async function addLabel() {
		addErr = '';
		if (!newName.trim()) {
			addErr = 'Label name is required.';
			return;
		}
		adding = true;
		try {
			await api.createLabel({ name: newName.trim(), color: newColor, group: newGroup || undefined });
			newName = '';
			await load();
			showToast('Label added');
		} catch (e) {
			addErr = e.message || 'Could not add label.';
		} finally {
			adding = false;
		}
	}

	let dragId = $state(null);
	let overName = $state('');

	function dragStart(e, l) {
		dragId = l.id;
		e.dataTransfer.setData('text/plain', l.id);
		e.dataTransfer.effectAllowed = 'move';
	}
	function dragEnd() {
		dragId = null;
		overName = '';
	}
	function dragOver(e, row) {
		if (!dragId) return;
		e.preventDefault();
		e.dataTransfer.dropEffect = 'move';
		overName = row.name;
	}
	function dragLeave(e) {
		// dragleave also fires at child boundaries; only clear when the pointer
		// left the section for real.
		if (!e.currentTarget.contains(e.relatedTarget)) overName = '';
	}
	async function drop(e, row) {
		e.preventDefault();
		overName = '';
		const id = e.dataTransfer.getData('text/plain') || dragId;
		dragId = null;
		const label = labels.find((l) => l.id === id);
		if (!label) return;
		const groupId = row.id;
		if ((label.groupId ?? null) === groupId) return;
		const prev = label.groupId;
		label.groupId = groupId;
		try {
			await api.moveLabel(id, groupId);
			sharedLabels.update((ls) => ls.map((x) => (x.id === id ? { ...x, groupId } : x)));
		} catch (e2) {
			label.groupId = prev;
			showToast('Could not move label: ' + (e2?.message || e2), 'error');
		}
	}
</script>

<div class="settings-wrap">
	<div class="settings-head">
		<h1 class="settings-h1">Labels</h1>
		<p class="settings-lede">Exclusive groups — one label per group, per issue.</p>
	</div>

	{#if loading}
		<p class="settings-hint">Loading…</p>
	{:else}
		{#each groupRows as row (row.name)}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<section
				class="settings-panel"
				class:drop-hint={overName === row.name}
				ondragover={(e) => dragOver(e, row)}
				ondragleave={dragLeave}
				ondrop={(e) => drop(e, row)}
			>
				<div class="settings-panel-h">
					{row.name === 'ungrouped' ? '(no group)' : row.name}{#if row.exclusive}<span class="n">exclusive</span>{/if}
				</div>
				<div class="settings-inline">
					{#each row.items as l (l.id)}
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<span
							class="pill"
							class:dragging={dragId === l.id}
							draggable="true"
							ondragstart={(e) => dragStart(e, l)}
							ondragend={dragEnd}
						><span class="dot" style="background:{l.color}"></span>{l.name}</span>
					{:else}
						<span class="settings-hint">No labels yet.</span>
					{/each}
					<button class="btn ghost settings-btn-sm" onclick={() => startAdd(row.name === 'ungrouped' ? '' : row.name)}>+ Add label</button>
				</div>
			</section>
		{/each}
	{/if}

	<section class="settings-panel">
		<div class="settings-panel-h">Add a label</div>
		<div class="settings-inline">
			<select class="select settings-input-short" bind:value={newGroup}>
				<option value="">(no group)</option>
				{#each groupNames as g (g)}
					<option value={g}>{g}</option>
				{/each}
			</select>
			<input id="label-name-input" class="input" style="width:180px" placeholder="label name" bind:value={newName} />
			<div class="settings-swatches">
				{#each SWATCHES as hex (hex)}
					<button
						type="button"
						class="settings-swatch"
						class:on={newColor === hex}
						style="background:{hex}"
						aria-label={`Use ${hex}`}
						onclick={() => (newColor = hex)}
					></button>
				{/each}
			</div>
			<button class="btn primary" onclick={addLabel} disabled={adding}>{adding ? 'Adding…' : 'Add'}</button>
		</div>
		{#if addErr}<span class="settings-err">{addErr}</span>{/if}
	</section>
</div>
