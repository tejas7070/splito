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

function startSearch(query: string) {
  suggestions.value = [];
  nearbyUnavailable.value = false;
  searchError.value = '';
  activeController?.abort();
  if (debounceTimer) clearTimeout(debounceTimer);

  const trimmed = query.trim();
  if (trimmed.length < 3) {
    isSearching.value = false;
    isOpen.value = false;
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
  }, 400);
}

watch(() => props.modelValue, (value) => {
  if (value !== inputValue.value) {
    inputValue.value = value;
    isOpen.value = false;
    suggestions.value = [];
    isSearching.value = false;
    searchError.value = '';
    if (debounceTimer) clearTimeout(debounceTimer);
    activeController?.abort();
  }
});

function choosePlace(place: PlaceSuggestion) {
  inputValue.value = place.name;
  emit('update:modelValue', place.name);
  emit('select', place);
  isOpen.value = false;
  suggestions.value = [];
  if (debounceTimer) clearTimeout(debounceTimer);
  activeController?.abort();
  isSearching.value = false;
}

function updateInput(event: Event) {
  const value = (event.target as HTMLInputElement).value;
  inputValue.value = value;
  emit('update:modelValue', value);
  emit('select', null);
  startSearch(value);
}

function clearInput() {
  inputValue.value = '';
  emit('update:modelValue', '');
  emit('select', null);
  isOpen.value = false;
  suggestions.value = [];
  if (debounceTimer) clearTimeout(debounceTimer);
  activeController?.abort();
  isSearching.value = false;
}

function closeSuggestions() {
  window.setTimeout(() => {
    isOpen.value = false;
  }, 200);
}

onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer);
  activeController?.abort();
});
</script>

<template>
  <div class="location-autocomplete">
    <div class="location-input-shell" :class="{ 'is-focused': isOpen }">
      <i class="pi pi-map-marker location-leading-icon"></i>
      <input
        :value="inputValue"
        :placeholder="placeholder"
        :aria-label="ariaLabel"
        aria-autocomplete="list"
        :aria-expanded="isOpen"
        class="location-neu-input"
        autocomplete="off"
        @input="updateInput"
        @focus="isOpen = inputValue.trim().length >= 3 && suggestions.length > 0"
        @blur="closeSuggestions"
      />
      <button
        v-if="inputValue"
        type="button"
        class="location-clear-btn"
        title="Clear location"
        @click="clearInput"
      >
        <i class="pi pi-times"></i>
      </button>
      <i v-if="isSearching" class="pi pi-spin pi-spinner location-spinner-icon"></i>
    </div>

    <div v-if="isSearching" class="location-search-status">
      <i class="pi pi-spin pi-spinner mr-1"></i> Searching OpenStreetMap…
    </div>

    <ul v-if="isOpen && suggestions.length" class="location-suggestions" role="listbox">
      <li v-for="place in suggestions" :key="place.id" role="option">
        <button type="button" class="location-suggestion" @mousedown.prevent="choosePlace(place)">
          <div class="suggestion-icon-wrap" :class="{ 'is-nearby': place.nearby }">
            <i :class="place.nearby ? 'pi pi-compass' : 'pi pi-map-marker'"></i>
          </div>
          <span class="location-suggestion-copy">
            <span class="location-suggestion-name">{{ place.name }}</span>
            <small v-if="place.nearby" class="nearby-tag">
              <i class="pi pi-bolt mr-0.5"></i> Nearby<span v-if="place.distance_km"> · {{ place.distance_km }} km</span>
            </small>
            <small v-else class="match-tag">Matching place</small>
          </span>
        </button>
      </li>
      <li v-if="nearbyUnavailable" class="location-search-status">
        Nearby place search is unavailable; matching places are still shown.
      </li>
      <li class="location-attribution">Place data © OpenStreetMap contributors</li>
    </ul>

    <div v-else-if="searchError" class="location-search-status location-search-error">
      <i class="pi pi-exclamation-circle mr-1"></i> {{ searchError }}
    </div>
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

.location-input-shell {
  position: relative;
  width: 100%;
  display: flex;
  align-items: center;
}

.location-leading-icon {
  position: absolute;
  left: 13px;
  font-size: 13px;
  color: var(--accent, #38bdf8);
  pointer-events: none;
  z-index: 2;
  transition: transform 0.2s ease;
}

.location-input-shell.is-focused .location-leading-icon {
  transform: scale(1.15);
}

.location-neu-input {
  width: 100% !important;
  box-sizing: border-box !important;
  background: var(--surface, #0f172a) !important;
  color: var(--text-main, #f8fafc) !important;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08)) !important;
  border-radius: 12px !important;
  box-shadow: var(--shadow-inset, inset 0 2px 4px rgba(0, 0, 0, 0.3)) !important;
  padding: 11px 36px 11px 36px !important;
  font-size: 13px !important;
  font-family: inherit !important;
  outline: none !important;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1) !important;
}

.location-neu-input:focus {
  border-color: var(--accent, #38bdf8) !important;
  box-shadow: var(--shadow-inset, inset 0 2px 4px rgba(0, 0, 0, 0.3)), 0 0 0 2px rgba(56, 189, 248, 0.2) !important;
}

.location-neu-input::placeholder {
  color: var(--text-muted, #64748b) !important;
  font-size: 12.5px;
}

.location-clear-btn {
  position: absolute;
  right: 12px;
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  padding: 4px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 11px;
  transition: all 0.15s ease;
}

.location-clear-btn:hover {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.15);
}

.location-spinner-icon {
  position: absolute;
  right: 32px;
  font-size: 12px;
  color: var(--accent, #38bdf8);
}

.location-suggestions {
  position: absolute;
  inset: calc(100% + 6px) 0 auto;
  z-index: 50;
  max-height: 270px;
  overflow-y: auto;
  list-style: none;
  margin: 0;
  padding: 6px;
  border: 1px solid var(--border, rgba(125, 133, 146, 0.25));
  border-radius: 14px;
  background: var(--surface-overlay, #1e293b);
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.45), var(--shadow-flat-lg);
  backdrop-filter: blur(14px);
}

.location-suggestion {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--text-main, #f8fafc);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, transform 0.15s ease;
}

.location-suggestion:hover,
.location-suggestion:focus-visible {
  background: var(--active-pill, rgba(56, 189, 248, 0.12));
  outline: none;
  transform: translateX(2px);
}

.suggestion-icon-wrap {
  width: 28px;
  height: 28px;
  min-width: 28px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.05);
  color: var(--accent, #38bdf8);
  font-size: 12px;
}

.suggestion-icon-wrap.is-nearby {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.location-suggestion-copy {
  min-width: 0;
  display: grid;
  gap: 2px;
  flex: 1;
}

.location-suggestion-name {
  font-size: 13px;
  font-weight: 500;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.nearby-tag {
  color: #10b981;
  font-size: 11px;
  font-weight: 600;
}

.match-tag {
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
}

.location-search-status,
.location-attribution {
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
  line-height: 1.4;
  padding: 8px 12px;
}

.location-search-error {
  color: #ef4444;
}

.location-attribution {
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.06));
  text-align: right;
  font-size: 10px;
  color: var(--text-muted);
}

@media (max-width: 600px) {
  .location-neu-input {
    font-size: 13px !important;
    padding: 8px 26px 8px 28px !important;
    border-radius: 10px !important;
    min-height: 40px !important;
  }

  .location-leading-icon {
    left: 8px !important;
    font-size: 11px !important;
  }

  .location-clear-btn {
    right: 5px !important;
    width: 22px !important;
    height: 22px !important;
    font-size: 10px !important;
  }

  .location-spinner-icon {
    right: 7px !important;
    font-size: 11px !important;
  }

  .location-suggestions {
    max-height: 210px !important;
    padding: 4px !important;
    border-radius: 12px !important;
    z-index: 999 !important;
  }

  .location-suggestion {
    padding: 8px 10px !important;
    gap: 8px !important;
  }

  .suggestion-icon-wrap {
    width: 24px !important;
    height: 24px !important;
    min-width: 24px !important;
    font-size: 11px !important;
  }

  .location-suggestion-name {
    font-size: 12px !important;
    white-space: normal !important;
    display: -webkit-box !important;
    -webkit-line-clamp: 2 !important;
    -webkit-box-orient: vertical !important;
    overflow: hidden !important;
    line-height: 1.35 !important;
  }

  .nearby-tag, .match-tag {
    font-size: 10px !important;
  }
}
</style>
