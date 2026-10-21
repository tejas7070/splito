<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue';
import { locationService, type PlaceSuggestion } from '../api/services';

const props = defineProps<{
  modelValue: string;
  placeholder?: string;
  ariaLabel: string;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
  select: [place: PlaceSuggestion | null];
}>();

const suggestions = ref<PlaceSuggestion[]>([]);
const nearbyUnavailable = ref(false);
const searchError = ref('');
const isSearching = ref(false);
const isOpen = ref(false);
const inputValue = ref(props.modelValue);
let debounceTimer: ReturnType<typeof setTimeout> | undefined;
let activeController: AbortController | undefined;
let skipNextSearch = false;

watch(() => props.modelValue, (value) => {
  if (value !== inputValue.value) inputValue.value = value;
});

watch(inputValue, (query) => {
  if (skipNextSearch) {
    skipNextSearch = false;
    return;
  }
  suggestions.value = [];
  nearbyUnavailable.value = false;
  searchError.value = '';
  activeController?.abort();
  if (debounceTimer) clearTimeout(debounceTimer);

  const trimmed = query.trim();
  if (trimmed.length < 3) {
    isSearching.value = false;
    return;
  }

  isOpen.value = true;
  debounceTimer = setTimeout(async () => {
    const controller = new AbortController();
    activeController = controller;
    isSearching.value = true;
    try {
      const response = await locationService.search(trimmed, controller.signal);
      if (controller.signal.aborted) return;
      suggestions.value = response.data.suggestions;
      nearbyUnavailable.value = response.data.nearby_unavailable;
    } catch {
      if (controller.signal.aborted) return;
      searchError.value = 'Location search is temporarily unavailable. Please try again.';
    } finally {
      if (!controller.signal.aborted) isSearching.value = false;
    }
  }, 500);
});

function choosePlace(place: PlaceSuggestion) {
  skipNextSearch = inputValue.value !== place.name;
  inputValue.value = place.name;
  emit('update:modelValue', place.name);
  emit('select', place);
  isOpen.value = false;
  suggestions.value = [];
}

function updateInput(event: Event) {
  const value = (event.target as HTMLInputElement).value;
  inputValue.value = value;
  emit('update:modelValue', value);
  emit('select', null);
}

function closeSuggestions() {
  window.setTimeout(() => {
    isOpen.value = false;
  }, 150);
}

onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer);
  activeController?.abort();
});
</script>

<template>
  <div class="location-autocomplete">
    <input
      :value="inputValue"
      :placeholder="placeholder"
      :aria-label="ariaLabel"
      aria-autocomplete="list"
      :aria-expanded="isOpen"
      class="wizard-neu-input"
      autocomplete="off"
      @input="updateInput"
      @focus="isOpen = inputValue.trim().length >= 3 && suggestions.length > 0"
      @blur="closeSuggestions"
    />
    <div v-if="isSearching" class="location-search-status">Searching OpenStreetMap…</div>
    <ul v-if="isOpen && suggestions.length" class="location-suggestions" role="listbox">
      <li v-for="place in suggestions" :key="place.id" role="option">
        <button type="button" class="location-suggestion" @mousedown.prevent="choosePlace(place)">
          <i :class="place.nearby ? 'pi pi-map-marker' : 'pi pi-search'"></i>
          <span class="location-suggestion-copy">
            <span class="location-suggestion-name">{{ place.name }}</span>
            <small v-if="place.nearby">
              Nearby<span v-if="place.distance_km"> · {{ place.distance_km }} km</span>
            </small>
            <small v-else>Matching place</small>
          </span>
        </button>
      </li>
      <li v-if="nearbyUnavailable" class="location-search-status">
        Nearby place search is unavailable; matching places are still shown.
      </li>
      <li class="location-attribution">Place data © OpenStreetMap contributors</li>
    </ul>
    <div v-else-if="searchError" class="location-search-status location-search-error">{{ searchError }}</div>
    <div v-else-if="isOpen && !isSearching && inputValue.trim().length >= 3" class="location-search-status">
      No matching places found.
    </div>
  </div>
</template>

<style scoped>
.location-autocomplete {
  position: relative;
  width: 100%;
}

.location-suggestions {
  position: absolute;
  inset: calc(100% + 6px) 0 auto;
  z-index: 20;
  max-height: 260px;
  overflow-y: auto;
  list-style: none;
  margin: 0;
  padding: 6px;
  border: 1px solid var(--border, rgba(125, 133, 146, 0.25));
  border-radius: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-lg);
}

.location-suggestion {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
}

.location-suggestion:hover,
.location-suggestion:focus-visible {
  background: var(--active-pill);
  outline: none;
}

.location-suggestion > i {
  color: var(--accent);
  margin-top: 2px;
}

.location-suggestion-copy {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.location-suggestion-name {
  font-size: 13px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.location-suggestion-copy small,
.location-search-status,
.location-attribution {
  color: var(--text-muted);
  font-size: 11px;
  line-height: 1.4;
}

.location-search-status,
.location-attribution {
  padding: 8px 10px;
}

.location-search-error {
  color: #dc2626;
}

.location-attribution {
  border-top: 1px solid var(--border);
  text-align: right;
}
</style>
