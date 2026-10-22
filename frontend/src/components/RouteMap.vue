<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';

export interface RouteMapPoint {
  name: string;
  latitude: number;
  longitude: number;
  kind: 'start' | 'stop' | 'destination';
}

const props = defineProps<{ points: RouteMapPoint[] }>();
const mapElement = ref<HTMLDivElement | null>(null);
const routeNotice = ref('');
const routeLegs = ref<Array<{ from: string; to: string; distance: number }>>([]);
const isLoadingRoute = ref(false);
let map: L.Map | null = null;
let routeLayer: L.LayerGroup | null = null;
let requestController: AbortController | null = null;

interface RoutingResponse {
  code: string;
  routes?: Array<{
    geometry: { coordinates: number[][] };
    legs: Array<{ distance: number }>;
  }>;
}

const totalDistance = computed(() => routeLegs.value.reduce((total, leg) => total + leg.distance, 0));

function pointKey(points: RouteMapPoint[]) {
  return points.map(({ latitude, longitude, name, kind }) => `${latitude},${longitude},${name},${kind}`).join('|');
}

function formatDistance(meters: number) {
  if (meters < 1000) return `${Math.round(meters)} m`;
  return `${new Intl.NumberFormat('en-IN', { maximumFractionDigits: 1 }).format(meters / 1000)} km`;
}

async function renderRoute() {
  if (!map || !routeLayer || props.points.length < 2) return;
  requestController?.abort();
  const controller = new AbortController();
  requestController = controller;
  routeNotice.value = '';
  routeLegs.value = [];
  isLoadingRoute.value = true;
  routeLayer.clearLayers();

  const coordinates = props.points.map((point) => `${point.longitude},${point.latitude}`).join(';');
  let lineCoordinates: L.LatLngExpression[] = props.points.map((point) => [point.latitude, point.longitude]);
  try {
    const response = await fetch(
      `https://router.project-osrm.org/route/v1/driving/${coordinates}?overview=full&geometries=geojson&steps=false`,
      { signal: controller.signal },
    );
    if (!response.ok) throw new Error(`Routing returned HTTP ${response.status}`);
    const result = await response.json() as RoutingResponse;
    if (controller.signal.aborted) return;
    const route = result.routes?.[0];
    if (
      result.code !== 'Ok' ||
      !route?.geometry.coordinates.length ||
      route.legs.length !== props.points.length - 1 ||
      route.legs.some((leg) => !Number.isFinite(leg.distance) || leg.distance < 0)
    ) {
      throw new Error('No driving route was found');
    }
    lineCoordinates = route.geometry.coordinates.map(([longitude, latitude]) => [latitude, longitude]);
    routeLegs.value = route.legs.map((leg, index) => ({
      from: props.points[index].name,
      to: props.points[index + 1].name,
      distance: leg.distance,
    }));
  } catch {
    if (controller.signal.aborted) return;
    routeNotice.value = 'Road routing is unavailable; showing a direct-line preview instead.';
  } finally {
    if (!controller.signal.aborted) isLoadingRoute.value = false;
  }

  if (controller.signal.aborted || !map || !routeLayer) return;
  L.polyline(lineCoordinates, { color: '#0891b2', weight: 5, opacity: 0.8 }).addTo(routeLayer);
  props.points.forEach((point, index) => {
    const label = point.kind === 'start' ? 'S' : point.kind === 'destination' ? 'D' : String(index);
    const marker = L.divIcon({
      className: `route-marker route-marker-${point.kind}`,
      html: `<span>${label}</span>`,
      iconSize: [30, 30],
      iconAnchor: [15, 15],
    });
    const tooltip = document.createElement('span');
    tooltip.textContent = point.name;
    L.marker([point.latitude, point.longitude], { icon: marker, title: point.name })
      .bindTooltip(tooltip)
      .addTo(routeLayer!);
  });

  map.fitBounds(L.latLngBounds(lineCoordinates).pad(0.15), { maxZoom: 12 });
  window.setTimeout(() => map?.invalidateSize(), 0);
}

onMounted(() => {
  if (!mapElement.value) return;
  map = L.map(mapElement.value, { scrollWheelZoom: false });
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
  }).addTo(map);
  routeLayer = L.layerGroup().addTo(map);
  void renderRoute();
});

watch(() => pointKey(props.points), () => void renderRoute());

onBeforeUnmount(() => {
  requestController?.abort();
  map?.remove();
  map = null;
  routeLayer = null;
});
</script>

<template>
  <div class="route-map-shell">
    <div ref="mapElement" class="route-map"></div>
    <section
      v-if="isLoadingRoute || routeLegs.length"
      class="route-distance-summary"
      aria-live="polite"
      aria-label="Route distances"
    >
      <div class="route-distance-total">
        <span>{{ isLoadingRoute ? 'Calculating driving distances…' : 'Total driving distance' }}</span>
        <strong v-if="!isLoadingRoute">{{ formatDistance(totalDistance) }}</strong>
      </div>
      <ol v-if="!isLoadingRoute" class="route-distance-legs">
        <li v-for="(leg, index) in routeLegs" :key="`${leg.from}-${leg.to}-${index}`">
          <span class="route-distance-index">{{ index + 1 }}</span>
          <span class="route-distance-names">
            <span>{{ leg.from }}</span>
            <span aria-hidden="true">→</span>
            <span>{{ leg.to }}</span>
          </span>
          <strong>{{ formatDistance(leg.distance) }}</strong>
        </li>
      </ol>
    </section>
    <p v-if="routeNotice" class="route-map-notice">{{ routeNotice }}</p>
    <small class="route-map-credit">Map tiles © OpenStreetMap contributors · Route service © OSRM</small>
  </div>
</template>

<style>
.route-map-shell {
  width: 100%;
}

.route-map {
  min-height: 280px;
  height: min(36vh, 380px);
  overflow: hidden;
  border: 1px solid var(--border, rgba(125, 133, 146, 0.25));
  border-radius: 14px;
  z-index: 0;
}

.route-marker {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px solid white;
  border-radius: 50%;
  color: white;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35);
}

.route-marker-start {
  background: #059669;
}

.route-marker-stop {
  background: #d97706;
}

.route-marker-destination {
  background: #e11d48;
}

.route-map-notice,
.route-map-credit {
  display: block;
  margin: 6px 2px 0;
  color: var(--text-muted);
  font-size: 11px;
}

.route-distance-summary {
  margin-top: 10px;
  padding: 12px;
  border-radius: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  color: var(--text-main);
  font-size: 12px;
}

.route-distance-total {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.route-distance-total > span {
  color: var(--text-muted);
}

.route-distance-total strong,
.route-distance-legs strong {
  color: var(--text-main);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.route-distance-legs {
  display: grid;
  gap: 8px;
  margin: 10px 0 0;
  padding: 10px 0 0;
  border-top: 1px solid var(--border-subtle, rgba(125, 133, 146, 0.2));
  list-style: none;
}

.route-distance-legs li {
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
}

.route-distance-index {
  display: grid;
  width: 20px;
  height: 20px;
  place-items: center;
  border-radius: 50%;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  color: var(--text-muted);
  font-size: 10px;
  font-weight: 700;
}

.route-distance-names {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px;
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
