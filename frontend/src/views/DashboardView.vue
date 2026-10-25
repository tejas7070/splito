<script setup lang="ts">
import { ref, computed, onMounted, defineAsyncComponent } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { useThemeStore } from '../stores/theme';
import { useToast } from '../stores/toast';
import { tripService, claimService, accountingService, type Trip, type DashboardSummary, type TripInvitation, type EligibleUser, type PlaceSuggestion } from '../api/services';
import { useOfflineSync } from '../services/offlineSync';
import appFavicon from '../assets/favicon.png';
import LocationAutocomplete from '../components/LocationAutocomplete.vue';
import type { RouteMapPoint } from '../components/RouteMap.vue';

import Button from 'primevue/button';
import Dialog from 'primevue/dialog';
import Tag from 'primevue/tag';
import InputText from 'primevue/inputtext';
import InputNumber from 'primevue/inputnumber';
import Textarea from 'primevue/textarea';
import Message from 'primevue/message';
import Avatar from 'primevue/avatar';
import Popover from 'primevue/popover';
import Divider from 'primevue/divider';
import AutoComplete from 'primevue/autocomplete';
import Password from 'primevue/password';
import Select from 'primevue/select';

const RouteMap = defineAsyncComponent(() => import('../components/RouteMap.vue'));

const authStore = useAuthStore();
const themeStore = useThemeStore();
const toast = useToast();
const offlineSync = useOfflineSync();
const router = useRouter();
const route = useRoute();

const trips = ref<Trip[]>([]);
const selectedTrip = ref<Trip | null>(null);
const dashboard = ref<DashboardSummary | null>(null);
const loading = ref(false);
const mobileSidebarOpen = ref(false);

const isCreatingTrip = ref(false);
const editingTripId = ref<number | null>(null);
const wizardStep = ref<1 | 2 | 3 | 4>(1);
const creatingTripLoading = ref(false);
const createdTripResult = ref<Trip | null>(null);

interface EditableStop {
  id: string;
  input: string;
  place: PlaceSuggestion | null;
}

let stopIdCounter = 0;
const createStopId = () => `stop-${Date.now()}-${++stopIdCounter}`;

const wizardForm = ref({
  title: '',
  start_point: '',
  start_place: null as PlaceSuggestion | null,
  destination: '',
  destination_place: null as PlaceSuggestion | null,
  stops: [] as EditableStop[],
  start_at: '',
  end_at: '',
  estimated_budget: 50000 as number | null,
});

const wizardHeading = computed(() => editingTripId.value ? 'Edit Trip Route' : 'Plan New Trip');
const routeMapPoints = computed<RouteMapPoint[]>(() => {
  const { start_place, destination_place, stops } = wizardForm.value;
  if (!start_place || !destination_place || stops.some((stop) => !stop.place)) return [];
  return [
    { ...start_place, kind: 'start' },
    ...stops.map((stop) => ({ ...stop.place!, kind: 'stop' as const })),
    { ...destination_place, kind: 'destination' },
  ];
});

const openCreateTripWizard = () => {
  editingTripId.value = null;
  wizardForm.value = {
    title: '',
    start_point: '',
    start_place: null,
    destination: '',
    destination_place: null,
    stops: [],
    start_at: '',
    end_at: '',
    estimated_budget: 50000,
  };
  wizardStep.value = 1;
  createdTripResult.value = null;
  isCreatingTrip.value = true;
};

const cancelCreateTripWizard = () => {
  isCreatingTrip.value = false;
  wizardStep.value = 1;
  createdTripResult.value = null;
  editingTripId.value = null;
};

const hasDisallowedChars = (val: string) => {
  return /[<>;'"]|--|\/\*|\*\//.test(val);
};

const openEditTripWizard = async () => {
  if (!selectedTrip.value) return;
  creatingTripLoading.value = true;
  try {
    const { data } = await tripService.getDetail(selectedTrip.value.id);
    const trip = data.trip;
    const toPlace = (name: string, latitude?: number | null, longitude?: number | null): PlaceSuggestion | null => {
      if (latitude == null || longitude == null) return null;
      return { id: `saved-${latitude}-${longitude}`, name, latitude, longitude, nearby: false };
    };
    const toLocalDateTime = (value: string) => {
      const date = new Date(value);
      return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
    };

    editingTripId.value = trip.id;
    wizardForm.value = {
      title: trip.title || trip.name || '',
      start_point: trip.start_point || trip.start_location || '',
      start_place: toPlace(trip.start_point || trip.start_location || '', trip.start_latitude, trip.start_longitude),
      destination: trip.destination,
      destination_place: toPlace(trip.destination, trip.destination_latitude, trip.destination_longitude),
      stops: (trip.stops || []).map((stop, sIdx) => ({
        id: `saved-stop-${stop.id || sIdx}-${++stopIdCounter}`,
        input: stop.name,
        place: { id: `saved-stop-${stop.id}`, name: stop.name, latitude: stop.latitude, longitude: stop.longitude, nearby: false },
      })),
      start_at: toLocalDateTime(trip.start_at),
      end_at: toLocalDateTime(trip.end_at),
      estimated_budget: trip.estimated_budget ?? 0,
    };
    wizardStep.value = 1;
    createdTripResult.value = null;
    isCreatingTrip.value = true;
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Unable to load trip details for editing');
  } finally {
    creatingTripLoading.value = false;
  }
};

const isStep1Valid = computed(() => {
  const t = wizardForm.value.title?.trim() || '';
  return Boolean(t.length >= 3 && !hasDisallowedChars(t));
});

const dateValidationError = computed(() => {
  if (!wizardForm.value.start_at || !wizardForm.value.end_at) return '';
  const start = new Date(wizardForm.value.start_at).getTime();
  const end = new Date(wizardForm.value.end_at).getTime();
  if (isNaN(start) || isNaN(end)) return '';
  if (end <= start) {
    return 'End Date & Time must be strictly after Start Date & Time';
  }
  return '';
});

const tripDurationSummary = computed(() => {
  if (!wizardForm.value.start_at || !wizardForm.value.end_at) return '';
  const start = new Date(wizardForm.value.start_at).getTime();
  const end = new Date(wizardForm.value.end_at).getTime();
  if (isNaN(start) || isNaN(end) || end <= start) return '';

  const diffMs = end - start;
  const totalMins = Math.floor(diffMs / (1000 * 60));
  const days = Math.floor(totalMins / (24 * 60));
  const hours = Math.floor((totalMins % (24 * 60)) / 60);
  const mins = totalMins % 60;

  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (mins > 0 || parts.length === 0) parts.push(`${mins}m`);

  const startDate = new Date(wizardForm.value.start_at);
  const endDate = new Date(wizardForm.value.end_at);
  const start24 = startDate.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', hour12: false });
  const end24 = endDate.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', hour12: false });

  return `Trip Duration: ${parts.join(' ')} (${start24} hrs → ${end24} hrs)`;
});

const isStep2Valid = computed(() => {
  const f = wizardForm.value;
  const startP = f.start_point?.trim() || '';
  const dest = f.destination?.trim() || '';
  const startT = f.start_at ? new Date(f.start_at).getTime() : 0;
  const endT = f.end_at ? new Date(f.end_at).getTime() : 0;
  return Boolean(
    startP.length >= 2 && f.start_place &&
    dest.length >= 2 && f.destination_place &&
    f.stops.every((stop) => stop.place) &&
    f.start_at && f.end_at &&
    !isNaN(startT) && !isNaN(endT) && endT > startT
  );
});

// Drag and drop shuffling for trip stops
const draggedStopIndex = ref<number | null>(null);
const dragOverStopIndex = ref<number | null>(null);

const onStopDragStart = (index: number, event: DragEvent) => {
  draggedStopIndex.value = index;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', String(index));
  }
};

const onStopDragOver = (index: number, event: DragEvent) => {
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move';
  }
  dragOverStopIndex.value = index;
};

const onStopDrop = (index: number) => {
  if (draggedStopIndex.value !== null && draggedStopIndex.value !== index) {
    const stops = wizardForm.value.stops;
    const moved = stops.splice(draggedStopIndex.value, 1)[0];
    stops.splice(index, 0, moved);
  }
  draggedStopIndex.value = null;
  dragOverStopIndex.value = null;
};

const onStopDragEnd = () => {
  draggedStopIndex.value = null;
  dragOverStopIndex.value = null;
};

// Touch drag handlers for mobile devices
const onTouchDragStart = (index: number) => {
  draggedStopIndex.value = index;
};

const onTouchDragMove = (event: TouchEvent) => {
  if (draggedStopIndex.value === null) return;
  const touch = event.touches[0];
  const targetEl = document.elementFromPoint(touch.clientX, touch.clientY);
  const rowEl = targetEl?.closest('.trip-stop-row');
  if (rowEl) {
    const targetIdx = Number(rowEl.getAttribute('data-index'));
    if (!isNaN(targetIdx) && targetIdx !== dragOverStopIndex.value) {
      dragOverStopIndex.value = targetIdx;
    }
  }
};

const onTouchDragEnd = () => {
  if (draggedStopIndex.value !== null && dragOverStopIndex.value !== null && draggedStopIndex.value !== dragOverStopIndex.value) {
    onStopDrop(dragOverStopIndex.value);
  }
  draggedStopIndex.value = null;
  dragOverStopIndex.value = null;
};

// Route distance state emitted from RouteMap
const routeDistanceInfo = ref<{
  totalDistance: number;
  legs: Array<{ from: string; to: string; distance: number }>;
  isLoading: boolean;
  error: string;
} | null>(null);

const onRouteCalculated = (info: {
  totalDistance: number;
  legs: Array<{ from: string; to: string; distance: number }>;
  isLoading: boolean;
  error: string;
}) => {
  routeDistanceInfo.value = info;
};

const formatDistanceKm = (meters: number) => {
  if (!meters) return '0 km';
  if (meters < 1000) return `${Math.round(meters)} m`;
  return `${new Intl.NumberFormat('en-IN', { maximumFractionDigits: 1 }).format(meters / 1000)} km`;
};

const isStep3Valid = computed(() => {
  const f = wizardForm.value;
  return Boolean(f.estimated_budget !== null && f.estimated_budget > 0);
});

const goToWizardStep = (step: 1 | 2 | 3) => {
  if (step === 2 && !isStep1Valid.value) return;
  if (step === 3 && (!isStep1Valid.value || !isStep2Valid.value)) return;
  wizardStep.value = step;
};

const nextWizardStep = () => {
  if (wizardStep.value === 1) {
    if (hasDisallowedChars(wizardForm.value.title || '')) {
      toast.error('Trip name contains disallowed special characters (< > ; \' " -- /* */)');
      return;
    }
    if (!isStep1Valid.value) {
      toast.error('Please enter a trip name (at least 3 characters)');
      return;
    }
    wizardStep.value = 2;
  } else if (wizardStep.value === 2) {
    if (!isStep2Valid.value) {
      if (!wizardForm.value.start_point?.trim() || !wizardForm.value.destination?.trim()) {
        toast.error('Please provide both the starting point and destination');
      } else if (!wizardForm.value.start_place || !wizardForm.value.destination_place || wizardForm.value.stops.some((stop) => !stop.place)) {
        toast.error('Choose a suggested map location for the start, destination, and each stop');
      } else if (!wizardForm.value.start_at || !wizardForm.value.end_at) {
        toast.error('Please choose both start and end dates');
      } else if (new Date(wizardForm.value.start_at).getTime() >= new Date(wizardForm.value.end_at).getTime()) {
        toast.error('End date & time must be strictly after the start date & time');
      }
      return;
    }
    wizardStep.value = 3;
  }
};

const addTripStop = () => {
  if (wizardForm.value.stops.length >= 20) {
    toast.warn('A trip can have at most 20 stops');
    return;
  }
  wizardForm.value.stops.push({ id: createStopId(), input: '', place: null });
};

const removeTripStop = (index: number) => {
  wizardForm.value.stops.splice(index, 1);
};



const prevWizardStep = () => {
  if (wizardStep.value === 2) wizardStep.value = 1;
  else if (wizardStep.value === 3) wizardStep.value = 2;
};

const handleWizardSubmit = async () => {
  if (hasDisallowedChars(wizardForm.value.title)) {
    toast.error('Trip name contains disallowed characters.');
    return;
  }
  if (!isStep1Valid.value || !isStep2Valid.value || !isStep3Valid.value) {
    toast.error('Please fill in all required trip details before creating');
    return;
  }
  creatingTripLoading.value = true;
  try {
    const budgetVal = Number(wizardForm.value.estimated_budget || 0);
    const startPlace = wizardForm.value.start_place!;
    const destinationPlace = wizardForm.value.destination_place!;
    const tripPayload = {
      title: wizardForm.value.title.trim(),
      start_point: startPlace.name,
      start_latitude: startPlace.latitude,
      start_longitude: startPlace.longitude,
      destination: destinationPlace.name,
      destination_latitude: destinationPlace.latitude,
      destination_longitude: destinationPlace.longitude,
      start_at: new Date(wizardForm.value.start_at).toISOString(),
      end_at: new Date(wizardForm.value.end_at).toISOString(),
      estimated_budget: budgetVal,
      stops: wizardForm.value.stops.map((stop) => ({
        name: stop.place!.name,
        latitude: stop.place!.latitude,
        longitude: stop.place!.longitude,
      })),
    };

    if (editingTripId.value) {
      const res = await tripService.update({ ...tripPayload, trip_id: editingTripId.value });
      const updatedTrip = res.data;
      editingTripId.value = null;
      isCreatingTrip.value = false;
      wizardStep.value = 1;
      await loadTrips(false);
      await selectTrip(updatedTrip);
      toast.success(`Trip "${updatedTrip.title || updatedTrip.name}" updated successfully!`);
      return;
    }

    const res = await tripService.create(tripPayload);
    const newTrip = res.data;
    createdTripResult.value = newTrip;
    // Refresh trips list in background without auto-selecting away from wizard
    await loadTrips(false);
    wizardStep.value = 4; // Move to Success Celebration Deck
    toast.success(`Trip "${newTrip.title || newTrip.name}" created successfully!`);
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to create trip');
  } finally {
    creatingTripLoading.value = false;
  }
};

const handleRedirectToCreatedTrip = async () => {
  if (createdTripResult.value) {
    const target = trips.value.find(t => t.id === createdTripResult.value?.id) || createdTripResult.value;
    isCreatingTrip.value = false;
    wizardStep.value = 1;
    await selectTrip(target);
    activeTab.value = 'ledger';
    window.scrollTo({ top: 0, behavior: 'smooth' });
  } else {
    isCreatingTrip.value = false;
    wizardStep.value = 1;
    editingTripId.value = null;
  }
};
const showClaimModal = ref(false);
const showInviteModal = ref(false);
const showRejectModal = ref(false);
const rejectingClaimId = ref<number | null>(null);
const rejectionReason = ref('');
const claimReceiptFile = ref<File | null>(null);
const uploadingReceipt = ref(false);

const generatingReport = ref(false);
const activeTab = ref<'ledger' | 'claims' | 'members' | 'settlement'>('ledger');
const claimsList = ref<any[]>([]);
const membersList = ref<any[]>([]);
const contributionsList = ref<any[]>([]);
const expensesList = ref<any[]>([]);
const ledgerSubTab = ref<'contributions' | 'expenses'>('contributions');

const settlementData = ref<any | null>(null);
const loadingSettlement = ref(false);

const loadSettlement = async (tripId: number) => {
  loadingSettlement.value = true;
  try {
    const res = await tripService.getSettlement(tripId);
    settlementData.value = res.data;
  } catch (err: any) {
    console.error('Error loading settlement:', err);
  } finally {
    loadingSettlement.value = false;
  }
};

const showDeleteTripModal = ref(false);
const deletingTrip = ref(false);

// Receipt Lightbox Viewer State
const showReceiptModal = ref(false);
const activeReceipt = ref<{ title: string; claimant?: string; amount?: number; url: string; isPdf: boolean } | null>(null);

const getFullReceiptUrl = (path?: string) => {
  if (!path) return '';
  if (path.startsWith('http://') || path.startsWith('https://')) return path;
  const apiBase = import.meta.env.VITE_API_URL || '';
  return `${apiBase}${path.startsWith('/') ? '' : '/'}${path}`;
};

const openReceiptViewer = (claim: any) => {
  const rawPath = claim.receipt_path || claim.receipt_url;
  if (!rawPath) return;
  const fullUrl = getFullReceiptUrl(rawPath);
  const isPdf = rawPath.toLowerCase().endsWith('.pdf') || rawPath.toLowerCase().includes('.pdf');
  activeReceipt.value = {
    title: claim.title || 'Expense Claim Receipt',
    claimant: claim.claimant_name,
    amount: claim.amount_paise,
    url: fullUrl,
    isPdf,
  };
  showReceiptModal.value = true;
};

const statusOptions = [
  { label: 'Planned', value: 'planned', icon: 'pi pi-calendar' },
  { label: 'Active', value: 'active', icon: 'pi pi-send' },
  { label: 'Completed', value: 'completed', icon: 'pi pi-check-circle' },
  { label: 'Cancelled', value: 'cancelled', icon: 'pi pi-times-circle' },
  { label: 'Archived', value: 'archived', icon: 'pi pi-box' },
];

const claimCategoryOptions = [
  { label: 'Fuel & Toll', value: 'Fuel & Toll', icon: 'pi pi-car' },
  { label: 'Food & Snacks', value: 'Food & Snacks', icon: 'pi pi-apple' },
  { label: 'Hotel & Stay', value: 'Hotel & Stay', icon: 'pi pi-building' },
  { label: 'Activities & Tickets', value: 'Activities & Tickets', icon: 'pi pi-ticket' },
  { label: 'Emergency & Other', value: 'Emergency & Other', icon: 'pi pi-shield' },
];

const claimPaymentMethodOptions = [
  { label: 'Cash in Hand', value: 'Cash', icon: 'pi pi-money-bill' },
  { label: 'UPI Payment', value: 'UPI', icon: 'pi pi-mobile' },
  { label: 'Debit / Credit Card', value: 'Card', icon: 'pi pi-credit-card' },
];

const rolePrivilegeOptions = [
  { label: 'Participant (Standard Member)', value: 'participant', icon: 'pi pi-user' },
  { label: 'Co-Reviewer (Can approve Munimji claims)', value: 'co_reviewer', icon: 'pi pi-check-circle' },
];

const contributionPaymentOptions = [
  { label: 'UPI Transfer', value: 'upi', icon: 'pi pi-mobile' },
  { label: 'Cash in Hand', value: 'cash', icon: 'pi pi-money-bill' },
  { label: 'Bank Transfer', value: 'bank_transfer', icon: 'pi pi-building' },
  { label: 'Other', value: 'other', icon: 'pi pi-ellipsis-h' },
];

const expenseCategoryOptions = [
  { label: 'Hotel & Stay', value: 'hotel', icon: 'pi pi-building' },
  { label: 'Petrol / Diesel', value: 'petrol', icon: 'pi pi-car' },
  { label: 'Food & Meals', value: 'food', icon: 'pi pi-apple' },
  { label: 'Toll & Parking', value: 'parking', icon: 'pi pi-map-pin' },
  { label: 'Travel & Transit', value: 'travel', icon: 'pi pi-send' },
  { label: 'Activities & Tickets', value: 'activities', icon: 'pi pi-ticket' },
  { label: 'Common Supplies', value: 'shopping', icon: 'pi pi-shopping-bag' },
  { label: 'Other Expense', value: 'other', icon: 'pi pi-ellipsis-h' },
];

const handleUpdateTripStatus = async (newStatus: 'planned' | 'active' | 'completed' | 'cancelled' | 'archived') => {
  if (!selectedTrip.value) return;
  try {
    await tripService.updateStatus(selectedTrip.value.id, newStatus);
    selectedTrip.value.status = newStatus;
    toast.success(`Trip status updated to ${newStatus}`);
    await loadTrips();
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to update trip status');
  }
};

const handleDeleteTrip = async () => {
  if (!selectedTrip.value) return;
  deletingTrip.value = true;
  try {
    const deletedTitle = selectedTrip.value.title;
    await tripService.deleteTrip(selectedTrip.value.id);
    toast.success(`Expedition "${deletedTitle}" deleted successfully`);
    showDeleteTripModal.value = false;
    selectedTrip.value = null;
    await loadTrips();
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to delete expedition');
  } finally {
    deletingTrip.value = false;
  }
};


const loadLedgerRecords = async (tripId: number) => {
  try {
    const [cRes, eRes] = await Promise.all([
      accountingService.listContributions(tripId),
      accountingService.listExpenses(tripId),
    ]);
    contributionsList.value = cRes.data || [];
    expensesList.value = eRes.data || [];
  } catch (err) {
    console.error('Error loading ledger records:', err);
  }
};

const handleGenerateReport = async () => {
  if (!selectedTrip.value) return;
  generatingReport.value = true;
  try {
    const res = await tripService.generateReport(selectedTrip.value.id);
    const blob = new Blob([res.data], { type: 'application/pdf' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `Splito_Report_${selectedTrip.value.title || 'Trip'}_${selectedTrip.value.id}.pdf`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
    toast.success('Report downloaded successfully!');
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to download report');
  } finally {
    generatingReport.value = false;
  }
};

const newInvite = ref({
  email: '',
  role: 'participant' as 'participant' | 'co_reviewer',
});

const selectedEligibleUser = ref<any>(null);
const eligibleUsers = ref<EligibleUser[]>([]);
const searchingEligibleUsers = ref(false);
let searchDebounceTimer: any = null;

const openInviteModal = async () => {
  if (!selectedTrip.value) return;
  newInvite.value = {
    email: '',
    role: 'participant',
  };
  selectedEligibleUser.value = null;
  eligibleUsers.value = [];
  showInviteModal.value = true;
  await fetchEligibleUsers('');
};

const handleSearchEligibleUsers = (event: { query: string }) => {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(async () => {
    await fetchEligibleUsers(event.query || '');
  }, 250);
};

const fetchEligibleUsers = async (query: string = '') => {
  if (!selectedTrip.value) return;
  searchingEligibleUsers.value = true;
  try {
    const res = await tripService.listEligibleUsers(selectedTrip.value.id, query);
    eligibleUsers.value = res.data || [];
  } catch (err) {
    console.error('Failed to load eligible users:', err);
    eligibleUsers.value = [];
  } finally {
    searchingEligibleUsers.value = false;
  }
};

const handleEligibleUserSelect = (event: any) => {
  const val = event.value;
  if (val && typeof val === 'object') {
    newInvite.value.email = val.email;
  } else if (typeof val === 'string') {
    newInvite.value.email = val.trim();
  }
};

const handleEligibleUserChange = () => {
  const val = selectedEligibleUser.value;
  if (val && typeof val === 'object') {
    newInvite.value.email = val.email;
  } else if (typeof val === 'string') {
    newInvite.value.email = val.trim();
  } else {
    newInvite.value.email = '';
  }
};

const selectedUserMeta = computed(() => {
  if (selectedEligibleUser.value && typeof selectedEligibleUser.value === 'object') {
    return selectedEligibleUser.value;
  }
  const emailToFind = newInvite.value.email || (typeof selectedEligibleUser.value === 'string' ? selectedEligibleUser.value : '');
  if (emailToFind) {
    const found = eligibleUsers.value.find(
      (u) => u.email.toLowerCase() === emailToFind.toLowerCase()
    );
    if (found) return found;
    if (emailToFind.includes('@')) {
      return { name: emailToFind.split('@')[0], email: emailToFind };
    }
  }
  return null;
});

const isInviteReady = computed(() => {
  const email = (typeof selectedEligibleUser.value === 'object' && selectedEligibleUser.value?.email)
    ? selectedEligibleUser.value.email
    : (newInvite.value.email || (typeof selectedEligibleUser.value === 'string' ? selectedEligibleUser.value : ''));
  return Boolean(email && email.trim().includes('@'));
});

const loadMembers = async (tripId: number) => {
  try {
    const res = await tripService.listMembers(tripId);
    membersList.value = res.data || [];
  } catch (err) {
    console.error('Error loading members:', err);
  }
};

const invitationsList = ref<TripInvitation[]>([]);

const loadInvitations = async (tripId: number) => {
  try {
    const res = await tripService.listInvitations(tripId);
    invitationsList.value = res.data || [];
  } catch (err) {
    console.error('Error loading invitations:', err);
  }
};

const myPendingInvitations = ref<TripInvitation[]>([]);

const loadMyPendingInvitations = async () => {
  try {
    const res = await tripService.listMyPendingInvitations();
    myPendingInvitations.value = res.data || [];
  } catch (err) {
    console.error('Error loading pending invitations:', err);
  }
};

const showContributionModal = ref(false);

const showExpenseModal = ref(false);

const newContribution = ref<{
  user_id: number;
  amount_rupees: number | null;
  payment_method: string;
  notes: string;
  received_at: string;
}>({
  user_id: 0,
  amount_rupees: null,
  payment_method: '',
  notes: '',
  received_at: new Date().toISOString().slice(0, 16),
});

const openContributionModal = () => {
  newContribution.value = {
    user_id: 0,
    amount_rupees: null,
    payment_method: '',
    notes: '',
    received_at: new Date().toISOString().slice(0, 16),
  };
  showContributionModal.value = true;
};

const newExpense = ref<{
  title: string;
  category: string;
  amount_rupees: number | null;
  paid_from: string;
  notes: string;
  expense_date: string;
}>({
  title: '',
  category: '',
  amount_rupees: null,
  paid_from: 'group_funds',
  notes: '',
  expense_date: new Date().toISOString().slice(0, 16),
});

const openExpenseModal = () => {
  newExpense.value = {
    title: '',
    category: '',
    amount_rupees: null,
    paid_from: 'group_funds',
    notes: '',
    expense_date: new Date().toISOString().slice(0, 16),
  };
  showExpenseModal.value = true;
};

const handleRecordContribution = async () => {
  if (!selectedTrip.value) return;
  if (!newContribution.value.user_id) {
    toast.error('Please select a member');
    return;
  }
  if (!newContribution.value.amount_rupees || newContribution.value.amount_rupees <= 0) {
    toast.error('Please enter a valid amount');
    return;
  }
  // If offline, save contribution locally in IndexedDB outbox
  if (!offlineSync.isOnline.value) {
    await offlineSync.recordOffline({
      trip_id: selectedTrip.value.id,
      action_type: 'contribution',
      endpoint: '/accounting/contributions/record',
      payload: {
        trip_id: selectedTrip.value.id,
        user_id: Number(newContribution.value.user_id),
        amount: newContribution.value.amount_rupees,
        amount_paise: Math.round(Number(newContribution.value.amount_rupees || 0) * 100),
        payment_method: newContribution.value.payment_method,
        received_at: new Date(newContribution.value.received_at).toISOString(),
        notes: newContribution.value.notes,
      },
    });
    showContributionModal.value = false;
    newContribution.value.notes = '';
    return;
  }

  try {
    await accountingService.recordContribution({
      trip_id: selectedTrip.value.id,
      user_id: Number(newContribution.value.user_id),
      amount: newContribution.value.amount_rupees,
      amount_paise: Math.round(Number(newContribution.value.amount_rupees || 0) * 100),
      payment_method: newContribution.value.payment_method,
      received_at: new Date(newContribution.value.received_at).toISOString(),
      notes: newContribution.value.notes,
    });
    showContributionModal.value = false;
    newContribution.value.notes = '';
    const res = await tripService.getDashboard(selectedTrip.value.id);
    dashboard.value = res.data;
    await loadLedgerRecords(selectedTrip.value.id);
    toast.success('Contribution logged successfully!');
  } catch (err: any) {
    if (!navigator.onLine || err.message === 'Network Error') {
      await offlineSync.recordOffline({
        trip_id: selectedTrip.value.id,
        action_type: 'contribution',
        endpoint: '/accounting/contributions/record',
        payload: {
          trip_id: selectedTrip.value.id,
          user_id: Number(newContribution.value.user_id),
          amount: newContribution.value.amount_rupees,
          amount_paise: Math.round(Number(newContribution.value.amount_rupees || 0) * 100),
          payment_method: newContribution.value.payment_method,
          received_at: new Date(newContribution.value.received_at).toISOString(),
          notes: newContribution.value.notes,
        },
      });
      showContributionModal.value = false;
      newContribution.value.notes = '';
    } else {
      toast.error(err.response?.data?.error || 'Failed to record contribution');
    }
  }
};

const totalContributionRupees = computed(() => Number(dashboard.value?.total_contributions_paise || 0) / 100);

const handleRecordExpense = async () => {
  if (!selectedTrip.value) return;
  if (!newExpense.value.title.trim()) {
    toast.error('Please enter an expense title');
    return;
  }
  if (!newExpense.value.category) {
    toast.error('Please select an expense category');
    return;
  }
  if (!newExpense.value.amount_rupees || newExpense.value.amount_rupees <= 0) {
    toast.error('Please enter a valid amount');
    return;
  }

  const expenseAmount = Number(newExpense.value.amount_rupees || 0);
  const totalContributions = totalContributionRupees.value;
  const isOverContributions = expenseAmount > totalContributions;

  // Validation toaster to let Munimji know if group expense is more than contribution total
  if (isOverContributions) {
    const deficit = expenseAmount - totalContributions;
    toast.warn(
      `⚠️ Treasury Warning: Group expense (₹${expenseAmount.toLocaleString('en-IN')}) exceeds total expedition contributions (₹${totalContributions.toLocaleString('en-IN')}) by ₹${deficit.toLocaleString('en-IN')}!`,
      7000
    );
  }

  // If offline or network fails, save offline in local IndexedDB outbox
  if (!offlineSync.isOnline.value) {
    await offlineSync.recordOffline({
      trip_id: selectedTrip.value.id,
      action_type: 'expense',
      endpoint: '/accounting/expenses/record',
      payload: {
        trip_id: selectedTrip.value.id,
        title: newExpense.value.title,
        category: newExpense.value.category,
        amount: newExpense.value.amount_rupees,
        amount_paise: Math.round(newExpense.value.amount_rupees * 100),
        paid_from: 'group_funds',
        expense_date: new Date(newExpense.value.expense_date).toISOString(),
        notes: newExpense.value.notes,
      },
    });
    showExpenseModal.value = false;
    newExpense.value.title = '';
    newExpense.value.notes = '';
    return;
  }

  try {
    await accountingService.recordExpense({
      trip_id: selectedTrip.value.id,
      title: newExpense.value.title,
      category: newExpense.value.category,
      amount: newExpense.value.amount_rupees,
      amount_paise: Math.round(newExpense.value.amount_rupees * 100),
      paid_from: 'group_funds',
      expense_date: new Date(newExpense.value.expense_date).toISOString(),
      notes: newExpense.value.notes,
    });
    showExpenseModal.value = false;
    newExpense.value.title = '';
    newExpense.value.notes = '';
    const res = await tripService.getDashboard(selectedTrip.value.id);
    dashboard.value = res.data;
    await loadLedgerRecords(selectedTrip.value.id);
    if (!isOverContributions) {
      toast.success('Group expense recorded successfully!');
    }
  } catch (err: any) {
    // If network failed midway, fallback to saving offline
    if (!navigator.onLine || err.message === 'Network Error') {
      await offlineSync.recordOffline({
        trip_id: selectedTrip.value.id,
        action_type: 'expense',
        endpoint: '/accounting/expenses/record',
        payload: {
          trip_id: selectedTrip.value.id,
          title: newExpense.value.title,
          category: newExpense.value.category,
          amount: newExpense.value.amount_rupees,
          amount_paise: Math.round(newExpense.value.amount_rupees * 100),
          paid_from: 'group_funds',
          expense_date: new Date(newExpense.value.expense_date).toISOString(),
          notes: newExpense.value.notes,
        },
      });
      showExpenseModal.value = false;
      newExpense.value.title = '';
      newExpense.value.notes = '';
    } else {
      toast.error(err.response?.data?.error || 'Failed to record expense');
    }
  }
};

const handleInviteMember = async () => {
  if (!selectedTrip.value) return;
  const targetEmail = (typeof selectedEligibleUser.value === 'object' && selectedEligibleUser.value?.email)
    ? selectedEligibleUser.value.email
    : (newInvite.value.email || (typeof selectedEligibleUser.value === 'string' ? selectedEligibleUser.value : '')).trim();

  if (!targetEmail || !targetEmail.includes('@')) {
    toast.error('Please select a registered user or enter a valid email address');
    return;
  }
  try {
    await tripService.inviteMember({
      trip_id: selectedTrip.value.id,
      email: targetEmail,
      role: newInvite.value.role,
    });
    showInviteModal.value = false;
    newInvite.value.email = '';
    selectedEligibleUser.value = null;
    await loadMembers(selectedTrip.value.id);
    await loadInvitations(selectedTrip.value.id);
    toast.success('Invitation sent via email successfully!');
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to send invitation');
  }
};

const showRemoveMemberModal = ref(false);
const removingMember = ref<any | null>(null);
const removalReason = ref('');
const processingRemoval = ref(false);

const openRemoveMemberModal = (member: any) => {
  removingMember.value = member;
  removalReason.value = '';
  showRemoveMemberModal.value = true;
};

const confirmRemoveMember = async () => {
  if (!selectedTrip.value || !removingMember.value) return;
  if (!removalReason.value.trim()) {
    toast.error('Please enter a reason for removal');
    return;
  }
  processingRemoval.value = true;
  try {
    await tripService.removeMember({
      trip_id: selectedTrip.value.id,
      user_id: Number(removingMember.value.user_id),
      reason: removalReason.value.trim(),
    });
    showRemoveMemberModal.value = false;
    toast.success(`Removed ${removingMember.value.user_name || removingMember.value.email} from expedition`);
    await loadMembers(selectedTrip.value.id);
    await loadInvitations(selectedTrip.value.id);
    const res = await tripService.getDashboard(selectedTrip.value.id);
    dashboard.value = res.data;
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to remove member');
  } finally {
    processingRemoval.value = false;
  }
};

const updatingRoleId = ref<number | null>(null);

const handleUpdateRole = async (member: any, newRole: 'co_reviewer' | 'participant') => {
  if (!selectedTrip.value) return;
  updatingRoleId.value = member.user_id;
  try {
    const res = await tripService.updateMemberRole({
      trip_id: selectedTrip.value.id,
      user_id: member.user_id,
      role: newRole,
    });
    const roleName = newRole === 'co_reviewer' ? 'Co-Reviewer (Auditor)' : 'Participant';
    toast.success(res.data?.message || `Updated ${member.user_name || member.email} to ${roleName}`);
    await loadMembers(selectedTrip.value.id);
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to update member role');
  } finally {
    updatingRoleId.value = null;
  }
};

const respondingInvitation = ref(false);

const isMunimji = computed(() => {
  if (!selectedTrip.value) return false;
  if (selectedTrip.value.user_role === 'munimji') return true;
  if (selectedTrip.value.created_by && selectedTrip.value.created_by === authStore.user?.id) return true;
  if (selectedTrip.value.creator_user_id && selectedTrip.value.creator_user_id === authStore.user?.id) return true;
  const myMember = membersList.value.find(
    (m: any) => m.user_id === authStore.user?.id || 
    (authStore.user?.email && m.email?.toLowerCase() === authStore.user?.email?.toLowerCase())
  );
  if (myMember && myMember.role === 'munimji') return true;
  return false;
});

const isCoReviewer = computed(() => {
  if (!selectedTrip.value) return false;
  if (selectedTrip.value.user_role === 'co_reviewer') return true;
  const myMember = membersList.value.find(
    (m: any) => m.user_id === authStore.user?.id || 
    (authStore.user?.email && m.email?.toLowerCase() === authStore.user?.email?.toLowerCase())
  );
  if (myMember && myMember.role === 'co_reviewer') return true;
  return false;
});

const canReviewClaim = (claim: any) => {
  if (!claim || claim.status !== 'pending') return false;
  // Claimant cannot review or approve their own claim (even if Munimji)
  if (claim.claimant_id === authStore.user?.id) return false;
  // Munimji or Co-Reviewer can review
  return isMunimji.value || isCoReviewer.value;
};

const handleRespondInvitation = async (tripId: number, action: 'accept' | 'reject') => {
  respondingInvitation.value = true;
  try {
    await tripService.respondInvitation(tripId, action);
    toast.success(action === 'accept' ? 'Invitation accepted! Welcome to the expedition!' : 'Invitation declined.');
    await loadMyPendingInvitations();
    await loadTrips();
    if (action === 'accept') {
      const updated = trips.value.find(t => t.id === tripId);
      if (updated) {
        await selectTrip(updated);
        activeTab.value = 'ledger';
      }
    } else {
      if (selectedTrip.value?.id === tripId) {
        const remaining = trips.value.filter(t => t.id !== tripId && t.user_status !== 'invited');
        if (remaining.length > 0) {
          await selectTrip(remaining[0]);
        } else {
          selectedTrip.value = null;
        }
      }
    }
  } catch (err: any) {
    toast.error(err.response?.data?.error || `Failed to ${action} invitation`);
  } finally {
    respondingInvitation.value = false;
  }
};


const newClaim = ref<{
  amount_rupees: number | null;
  category: string;
  reason: string;
  payment_method: string;
  expense_date: string;
}>({
  amount_rupees: null,
  category: '',
  reason: '',
  payment_method: '',
  expense_date: new Date().toISOString().slice(0, 16),
});

const openClaimModal = () => {
  newClaim.value = {
    amount_rupees: null,
    category: '',
    reason: '',
    payment_method: '',
    expense_date: new Date().toISOString().slice(0, 16),
  };
  claimReceiptFile.value = null;
  showClaimModal.value = true;
};

const loadClaims = async (tripId: number) => {
  try {
    const res = await claimService.list(tripId);
    claimsList.value = res.data || [];
  } catch (err) {
    console.error('Error loading claims:', err);
  }
};

const handleReceiptFileChange = (e: Event) => {
  const target = e.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    claimReceiptFile.value = target.files[0];
  } else {
    claimReceiptFile.value = null;
  }
};

const handleCreateClaim = async () => {
  if (!selectedTrip.value) return;
  if (!newClaim.value.amount_rupees || newClaim.value.amount_rupees <= 0) {
    toast.error('Please enter a valid claim amount');
    return;
  }
  if (!newClaim.value.category) {
    toast.error('Please select an expense category');
    return;
  }
  if (!newClaim.value.reason.trim()) {
    toast.error('Please provide a reason or description for the claim');
    return;
  }
  if (!newClaim.value.payment_method) {
    toast.error('Please select how this expense was paid');
    return;
  }
  // If offline, save claim locally in IndexedDB with receipt image blob
  if (!offlineSync.isOnline.value) {
    await offlineSync.recordOffline(
      {
        trip_id: selectedTrip.value.id,
        action_type: 'claim',
        endpoint: '/claims/create',
        payload: {
          trip_id: selectedTrip.value.id,
          amount_paise: Math.round(Number(newClaim.value.amount_rupees || 0) * 100),
          category: newClaim.value.category,
          reason: `${newClaim.value.reason.trim()} (${newClaim.value.payment_method})`,
          expense_date: new Date(newClaim.value.expense_date).toISOString(),
        },
      },
      claimReceiptFile.value || undefined
    );
    showClaimModal.value = false;
    newClaim.value = {
      amount_rupees: null,
      category: '',
      reason: '',
      payment_method: '',
      expense_date: new Date().toISOString().slice(0, 16),
    };
    claimReceiptFile.value = null;
    return;
  }

  try {
    let receiptUrl = '';
    if (claimReceiptFile.value) {
      uploadingReceipt.value = true;
      try {
        const upRes = await accountingService.uploadReceipt(claimReceiptFile.value);
        receiptUrl = upRes.data.receipt_path;
      } catch (upErr: any) {
        toast.error('Failed to upload receipt file');
        uploadingReceipt.value = false;
        return;
      } finally {
        uploadingReceipt.value = false;
      }
    }

    await claimService.create({
      trip_id: selectedTrip.value.id,
      amount_paise: Math.round(newClaim.value.amount_rupees * 100),
      category: newClaim.value.category,
      reason: `${newClaim.value.reason.trim()} (${newClaim.value.payment_method})`,
      expense_date: new Date(newClaim.value.expense_date).toISOString(),
      receipt_url: receiptUrl || undefined,
    });
    showClaimModal.value = false;
    newClaim.value = {
      amount_rupees: null,
      category: '',
      reason: '',
      payment_method: '',
      expense_date: new Date().toISOString().slice(0, 16),
    };
    claimReceiptFile.value = null;
    await loadClaims(selectedTrip.value.id);
    const res = await tripService.getDashboard(selectedTrip.value.id);
    dashboard.value = res.data;
    toast.success('Claim submitted successfully to Munimji!');
  } catch (err: any) {
    if (!navigator.onLine || err.message === 'Network Error') {
      await offlineSync.recordOffline(
        {
          trip_id: selectedTrip.value.id,
          action_type: 'claim',
          endpoint: '/claims/create',
          payload: {
            trip_id: selectedTrip.value.id,
            amount_paise: Math.round(Number(newClaim.value.amount_rupees || 0) * 100),
            category: newClaim.value.category,
            reason: `${newClaim.value.reason.trim()} (${newClaim.value.payment_method})`,
            expense_date: new Date(newClaim.value.expense_date).toISOString(),
          },
        },
        claimReceiptFile.value || undefined
      );
      showClaimModal.value = false;
      newClaim.value = {
        amount_rupees: null,
        category: '',
        reason: '',
        payment_method: '',
        expense_date: new Date().toISOString().slice(0, 16),
      };
      claimReceiptFile.value = null;
    } else {
      toast.error(err.response?.data?.error || 'Failed to submit claim');
    }
  }
};

const openRejectModal = (claimId: number) => {
  rejectingClaimId.value = claimId;
  rejectionReason.value = '';
  showRejectModal.value = true;
};

const confirmRejectClaim = async () => {
  if (!selectedTrip.value || !rejectingClaimId.value) return;
  if (!rejectionReason.value.trim()) {
    toast.error('Please enter a rejection reason');
    return;
  }
  try {
    await claimService.reject(selectedTrip.value.id, rejectingClaimId.value, rejectionReason.value);
    showRejectModal.value = false;
    toast.info('Claim rejected.');
    await loadClaims(selectedTrip.value.id);
    const res = await tripService.getDashboard(selectedTrip.value.id);
    dashboard.value = res.data;
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Rejection failed');
  }
};

const handleDecideClaim = async (claimId: number, approve: boolean) => {
  if (!selectedTrip.value) return;
  if (!approve) {
    openRejectModal(claimId);
    return;
  }
  try {
    await claimService.approve(selectedTrip.value.id, claimId, 'Approved by Munimji');
    toast.success('Claim approved!');
    await loadClaims(selectedTrip.value.id);
    const res = await tripService.getDashboard(selectedTrip.value.id);
    dashboard.value = res.data;
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Decision failed');
  }
};


const loadTrips = async (autoSelect: boolean = true) => {
  loading.value = true;
  try {
    const [tRes] = await Promise.all([
      tripService.list(),
      loadMyPendingInvitations(),
    ]);
    trips.value = tRes.data || [];
    
    // If we are currently in wizard mode, do NOT auto-select another trip to overwrite the wizard
    if (isCreatingTrip.value || !autoSelect) {
      return;
    }

    const inviteTripId = route.query.invite_trip ? Number(route.query.invite_trip) : null;
    const targetTrip = inviteTripId ? trips.value.find(t => t.id === inviteTripId) : null;

    if (targetTrip) {
      await selectTrip(targetTrip);
    } else if (trips.value.length > 0) {
      if (selectedTrip.value) {
        const current = trips.value.find(t => t.id === selectedTrip.value?.id);
        if (current) {
          await selectTrip(current);
        } else {
          const active = trips.value.find(t => t.user_status !== 'invited');
          await selectTrip(active || trips.value[0]);
        }
      } else {
        const active = trips.value.find(t => t.user_status !== 'invited');
        await selectTrip(active || trips.value[0]);
      }
    } else {
      selectedTrip.value = null;
    }
  } catch (err) {
    console.error(err);
  } finally {
    loading.value = false;
  }
};


const selectTrip = async (trip: Trip) => {
  isCreatingTrip.value = false;
  mobileSidebarOpen.value = false;
  try {
    const detail = await tripService.getDetail(trip.id);
    selectedTrip.value = detail.data.trip;
    const res = await tripService.getDashboard(trip.id);
    dashboard.value = res.data;
    await loadClaims(trip.id);
    await loadMembers(trip.id);
    await loadInvitations(trip.id);
    await loadLedgerRecords(trip.id);
    await loadSettlement(trip.id);
  } catch (err) {
    console.error(err);
  }
};



const formatINR = (paise: number) => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(paise / 100);
};

const userInitials = computed(() => {
  if (!authStore.user) return 'U';
  if (authStore.user.full_name && authStore.user.full_name.trim()) {
    const parts = authStore.user.full_name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return parts[0].slice(0, 2).toUpperCase();
  }
  if (authStore.user.email) {
    return authStore.user.email.slice(0, 2).toUpperCase();
  }
  return 'U';
});

const profilePopover = ref();
const toggleProfilePopover = (event: Event) => {
  if (profilePopover.value) {
    profilePopover.value.toggle(event);
  }
};

const copyEmail = () => {
  if (authStore.user?.email) {
    navigator.clipboard.writeText(authStore.user.email);
    toast.success('Email copied to clipboard!');
  }
};

const logout = async () => {
  if (profilePopover.value) {
    profilePopover.value.hide();
  }
  await authStore.logout();
  toast.info('You have been signed out.');
  router.replace('/');
};

// --- User Profile & Password Dialog States ---
const showProfileModal = ref(false);
const profileName = ref('');
const profilePhone = ref('');
const savingProfile = ref(false);

const showPasswordModal = ref(false);
const currentPassword = ref('');
const newPassword = ref('');
const confirmNewPassword = ref('');
const changingPassword = ref(false);

const openProfileModal = () => {
  if (profilePopover.value) {
    profilePopover.value.hide();
  }
  profileName.value = authStore.user?.full_name || '';
  profilePhone.value = (authStore.user as any)?.phone || '';
  showProfileModal.value = true;
};

const openPasswordModal = () => {
  if (profilePopover.value) {
    profilePopover.value.hide();
  }
  currentPassword.value = '';
  newPassword.value = '';
  confirmNewPassword.value = '';
  showPasswordModal.value = true;
};

const handleUpdateProfile = async () => {
  if (!profileName.value.trim()) {
    toast.error('Please enter your full name');
    return;
  }
  savingProfile.value = true;
  try {
    await authStore.updateProfile({
      name: profileName.value.trim(),
      phone: profilePhone.value.trim() || undefined,
    });
    toast.success('Profile updated successfully!');
    showProfileModal.value = false;
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to update profile');
  } finally {
    savingProfile.value = false;
  }
};

const handleChangePassword = async () => {
  if (!currentPassword.value) {
    toast.error('Please enter your current password');
    return;
  }
  if (newPassword.value.length < 6) {
    toast.error('New password must be at least 6 characters long');
    return;
  }
  if (newPassword.value !== confirmNewPassword.value) {
    toast.error('New passwords do not match');
    return;
  }

  changingPassword.value = true;
  try {
    await authStore.changePassword({
      current_password: currentPassword.value,
      new_password: newPassword.value,
    });
    toast.success('Password updated successfully!');
    currentPassword.value = '';
    newPassword.value = '';
    confirmNewPassword.value = '';
    showPasswordModal.value = false;
  } catch (err: any) {
    toast.error(err.response?.data?.error || 'Failed to change password');
  } finally {
    changingPassword.value = false;
  }
};

onMounted(async () => {
  // Hydrate user profile from backend via HttpOnly session
  await authStore.fetchUser();
  loadTrips();
});
</script>

<template>
  <div class="app-container">
    <!-- Clean Production-Grade Header -->
    <header class="app-header">
      <div class="header-start">
        <button @click="mobileSidebarOpen = !mobileSidebarOpen" class="neu-btn mobile-menu-btn" aria-label="Toggle Trips">
          <i class="pi pi-bars"></i>
        </button>

        <div class="brand-cluster">
          <div class="brand-icon-badge">
            <img :src="appFavicon" alt="Splito Logo" class="brand-logo-img" />
          </div>
          <span class="brand-title brand-font">Splito</span>
        </div>

        <span class="brand-divider">/</span>
        <div class="header-breadcrumb-tag">
          <span class="header-caption">Treasury Protocol</span>
          <span v-if="selectedTrip" class="header-trip-pill brand-font">{{ selectedTrip.title }}</span>
        </div>
      </div>

      <div class="header-actions">
        <!-- Offline & Sync Status Pill -->
        <div v-if="!offlineSync.isOnline.value || offlineSync.pendingCount.value > 0" class="flex items-center gap-2">
          <span
            :class="[
              'px-2.5 py-1 text-xs rounded-full font-medium flex items-center gap-1.5 transition-all shadow-sm',
              !offlineSync.isOnline.value ? 'bg-amber-950/70 border border-amber-600/50 text-amber-300' : 'bg-cyan-950/60 border border-cyan-500/40 text-cyan-300'
            ]"
            :title="!offlineSync.isOnline.value ? 'Working in offline mode' : `${offlineSync.pendingCount.value} pending offline records`"
          >
            <span class="w-2 h-2 rounded-full" :class="!offlineSync.isOnline.value ? 'bg-amber-400 animate-pulse' : 'bg-cyan-400'"></span>
            <span>{{ !offlineSync.isOnline.value ? 'Offline Mode' : `${offlineSync.pendingCount.value} Pending Sync` }}</span>
          </span>

          <button
            v-if="offlineSync.isOnline.value && offlineSync.pendingCount.value > 0"
            @click="offlineSync.syncPendingQueue(selectedTrip?.id)"
            :disabled="offlineSync.isSyncing.value"
            class="neu-btn px-2.5 py-1 text-xs font-semibold text-cyan-300 hover:text-white flex items-center gap-1"
            title="Sync offline records to server now"
          >
            <i class="pi pi-sync" :class="{ 'pi-spin': offlineSync.isSyncing.value }"></i>
            <span>{{ offlineSync.isSyncing.value ? 'Syncing...' : 'Sync Now' }}</span>
          </button>
        </div>

        <!-- New Trip -->
        <Button 
          @click="openCreateTripWizard" 
          label="Plan Trip" 
          icon="pi pi-plus" 
          size="small" 
          rounded 
          severity="primary" 
          class="btn-plan-header"
        />


        <!-- Theme Toggle -->
        <Button 
          @click="themeStore.toggleTheme" 
          :icon="themeStore.isDark ? 'pi pi-sun' : 'pi pi-moon'" 
          size="small" 
          rounded 
          text 
          severity="secondary" 
          :aria-label="themeStore.isDark ? 'Switch to light' : 'Switch to dark'"
        />

        <!-- User Profile Pill / Button -->
        <button 
          type="button" 
          @click="toggleProfilePopover" 
          class="user-profile-btn" 
          aria-haspopup="true" 
          aria-controls="profile_popover"
        >
          <Avatar 
            :label="userInitials" 
            shape="circle" 
            class="user-avatar-chip" 
          />
          <span class="user-email-text mono-num">{{ authStore.user?.email || 'My Account' }}</span>
          <i class="pi pi-chevron-down user-caret"></i>
        </button>

        <!-- Profile Popover Dropdown -->
        <Popover ref="profilePopover" id="profile_popover">
          <div class="popover-profile-card">
            <div class="popover-header-row">
              <Avatar 
                :label="userInitials" 
                size="large" 
                shape="circle" 
                class="popover-avatar-lg" 
              />
              <div class="popover-info-col">
                <span class="popover-display-name brand-font">
                  {{ authStore.user?.full_name || authStore.user?.email?.split('@')[0] || 'User Profile' }}
                </span>
                <span class="popover-display-email mono-num">{{ authStore.user?.email }}</span>
                <div class="popover-tag-row">
                  <Tag value="Signed In" severity="success" rounded />
                  <Tag v-if="authStore.user?.id" :value="'UID: ' + authStore.user?.id" severity="secondary" rounded />
                </div>
              </div>
            </div>

            <Divider class="popover-sep" />

            <div class="popover-menu-stack">
              <button class="popover-menu-btn" @click="openProfileModal">
                <i class="pi pi-user-edit"></i>
                <span>Edit Profile</span>
              </button>

              <button class="popover-menu-btn" @click="openPasswordModal">
                <i class="pi pi-key"></i>
                <span>Change Password</span>
              </button>

              <button class="popover-menu-btn" @click="copyEmail">
                <i class="pi pi-copy"></i>
                <span>Copy Email</span>
              </button>

              <button class="popover-menu-btn" @click="themeStore.toggleTheme">
                <i :class="themeStore.isDark ? 'pi pi-sun' : 'pi pi-moon'"></i>
                <span>{{ themeStore.isDark ? 'Switch to Light Theme' : 'Switch to Dark Theme' }}</span>
              </button>

              <Divider class="popover-sep" />

              <button class="popover-menu-btn text-danger" @click="logout">
                <i class="pi pi-sign-out"></i>
                <span>Sign Out</span>
              </button>
            </div>
          </div>
        </Popover>
      </div>
    </header>

    <!-- App Body (Responsive Grid/Flex) -->
    <div class="app-content-layout">
      <!-- Backdrop for Mobile Drawer -->
      <div 
        v-if="mobileSidebarOpen" 
        @click="mobileSidebarOpen = false" 
        class="mobile-drawer-backdrop"
      ></div>

      <!-- Left Column: Master Trips Drawer (Hidden when in Trip Wizard Mode) -->
      <aside v-if="!isCreatingTrip" :class="['sidebar-column', { 'drawer-open': mobileSidebarOpen }]">
        <!-- Mobile Drawer Header with Close Button -->
        <div class="mobile-drawer-top-bar">
          <div class="mobile-drawer-brand">
            <div class="brand-icon-badge small-badge">
              <img :src="appFavicon" alt="Splito Logo" class="brand-logo-img" />
            </div>
            <span class="brand-title brand-font">Splito</span>
          </div>
          <button @click="mobileSidebarOpen = false" class="mobile-drawer-close" aria-label="Close menu">
            <i class="pi pi-times"></i>
          </button>
        </div>

        <!-- 1. Dedicated Pending Invitations Section (if any) -->
        <div class="sidebar-invitations-group" v-if="myPendingInvitations.length > 0">

          <div class="sidebar-top pending-header">
            <span class="sidebar-label">📬 PENDING INVITATIONS</span>
            <span class="nav-count-pill">{{ myPendingInvitations.length }}</span>
          </div>
          <div class="sidebar-invitations-scroll">
            <div 
              v-for="inv in myPendingInvitations" 
              :key="'side-inv-' + inv.id"
              class="sidebar-invitation-card"
            >
              <div class="sidebar-inv-top">
                <span class="sidebar-inv-title brand-font">{{ inv.trip_title || 'Expedition' }}</span>
                <Tag 
                  :value="inv.role === 'co_reviewer' ? 'Co-Reviewer' : 'Participant'" 
                  :severity="inv.role === 'co_reviewer' ? 'info' : 'secondary'" 
                  rounded 
                />
              </div>
              <div class="sidebar-inv-route" v-if="inv.start_location && inv.destination">
                {{ inv.start_location }} → {{ inv.destination }}
              </div>
              <div class="sidebar-inv-actions">
                <Button 
                  @click.stop="handleRespondInvitation(inv.trip_id, 'accept')"
                  :loading="respondingInvitation"
                  label="Accept"
                  icon="pi pi-check"
                  severity="success"
                  size="small"
                />
                <Button 
                  @click.stop="handleRespondInvitation(inv.trip_id, 'reject')"
                  :loading="respondingInvitation"
                  label="Decline"
                  icon="pi pi-times"
                  severity="danger"
                  size="small"
                  text
                />
              </div>
            </div>
          </div>
        </div>

        <div class="sidebar-top">
          <span class="sidebar-label">ACTIVE TRIPS</span>
          <span class="nav-count-pill">{{ trips.filter(t => t.user_status !== 'invited').length }}</span>
        </div>

        <div class="trip-items-scroll">
          <div 
            v-for="trip in trips.filter(t => t.user_status !== 'invited')" 
            :key="trip.id"
            @click="selectTrip(trip)"
            :class="['trip-neu-item', { active: selectedTrip?.id === trip.id }]"
          >
            <div class="trip-item-row">
              <span class="trip-item-name brand-font">{{ trip.title }}</span>
              <span class="trip-item-status mono-num">{{ trip.status }}</span>
            </div>
            <div class="trip-item-route">
              {{ trip.start_point }} → {{ trip.destination }}
            </div>
          </div>

          <div v-if="trips.filter(t => t.user_status !== 'invited').length === 0" class="sidebar-empty-trips">
            <span>No active expeditions yet.</span>
          </div>
        </div>

        <!-- Section Navigation inside Sidebar (Clean, Organized) -->
        <div class="sidebar-section-nav" v-if="selectedTrip && selectedTrip.user_status !== 'invited'">
          <span class="sidebar-label">TRIP WORKSPACE</span>
          <div class="nav-pills-stack">
            <button 
              @click="activeTab = 'ledger'" 
              :class="['sidebar-nav-pill', { active: activeTab === 'ledger' }]"
            >
              <div class="sidebar-pill-content">
                <i class="pi pi-chart-bar"></i>
                <span>Treasury & Ledger</span>
              </div>
            </button>
            <button 
              @click="activeTab = 'claims'" 
              :class="['sidebar-nav-pill', { active: activeTab === 'claims' }]"
            >
              <div class="sidebar-pill-content">
                <i class="pi pi-receipt"></i>
                <span>Claims Protocol</span>
              </div>
              <span v-if="claimsList.length > 0" class="nav-count-pill">{{ claimsList.length }}</span>
            </button>
            <button 
              @click="activeTab = 'members'" 
              :class="['sidebar-nav-pill', { active: activeTab === 'members' }]"
            >
              <div class="sidebar-pill-content">
                <i class="pi pi-users"></i>
                <span>Members & Invite</span>
              </div>
              <span v-if="membersList.length > 0" class="nav-count-pill">{{ membersList.length }}</span>
            </button>
            <button 
              @click="activeTab = 'settlement'" 
              :class="['sidebar-nav-pill', { active: activeTab === 'settlement' }]"
            >
              <div class="sidebar-pill-content">
                <i class="pi pi-arrows-h"></i>
                <span>Settlement ("Who Owes")</span>
              </div>
              <span v-if="settlementData?.transfers?.length > 0" class="nav-count-pill">{{ settlementData.transfers.length }}</span>
            </button>
          </div>
        </div>
      </aside>

      <!-- Full-Width 30/70 Trip Wizard Viewport (Option C: Boarding Pass / Live Ticket Preview) -->
      <main class="wizard-split-layout" v-if="isCreatingTrip">
        <!-- Left 30%: Minimalist Boarding Pass / Trip Ticket -->
        <aside class="wizard-sidebar-visual wizard-ticket-preview-panel">
          <div class="ticket-header-pill">
            <span class="live-dot-pulse"></span>
            <span>LIVE BOARDING PASS</span>
          </div>

          <div class="pass-card">
            <!-- Top Stub -->
            <div class="pass-top-stub">
              <div class="pass-airline-brand">
                <div class="pass-logo-badge">
                  <i class="pi pi-compass"></i>
                </div>
                <div class="pass-brand-details">
                  <span class="pass-expedition-label">SPLITO GROUP VOYAGE</span>
                  <span class="pass-title brand-font">{{ wizardForm.title.trim() || 'Untitled Trip' }}</span>
                </div>
              </div>
              <Tag value="MUNIMJI LEDGER" severity="info" rounded class="pass-status-tag" />
            </div>

            <!-- Route Stub -->
            <div class="pass-route-deck">
              <div class="pass-route-city">
                <span class="pass-city-label">ORIGIN</span>
                <span class="pass-city-code">{{ (wizardForm.start_point.trim() ? wizardForm.start_point.trim().slice(0, 3).toUpperCase() : 'DEP') }}</span>
                <span class="pass-city-name">{{ wizardForm.start_point.trim() || 'Departure city' }}</span>
              </div>

              <div class="pass-flight-path">
                <i class="pi pi-arrow-right"></i>
                <div class="pass-flight-line"></div>
                <span class="pass-transport-mode">ROAD / EXPEDITION</span>
              </div>

              <div class="pass-route-city text-right">
                <span class="pass-city-label">DESTINATION</span>
                <span class="pass-city-code text-emerald-400">{{ (wizardForm.destination.trim() ? wizardForm.destination.trim().slice(0, 3).toUpperCase() : 'ARR') }}</span>
                <span class="pass-city-name">{{ wizardForm.destination.trim() || 'Destination city' }}</span>
              </div>
            </div>
            <div v-if="wizardForm.stops.length" class="pass-stop-list">
              VIA {{ wizardForm.stops.map((stop) => stop.place?.name || stop.input).filter(Boolean).join('  ·  ') }}
            </div>

            <!-- Ticket Perforation Divider -->
            <div class="pass-perforation-divider">
              <span class="perforation-notch left-notch"></span>
              <div class="perforation-dashed-line"></div>
              <span class="perforation-notch right-notch"></span>
            </div>

            <!-- Meta Details Grid -->
            <div class="pass-meta-grid">
              <div class="pass-meta-item">
                <span class="meta-field-label">START DATE</span>
                <span class="meta-field-val">
                  {{ wizardForm.start_at ? new Date(wizardForm.start_at).toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }}
                </span>
              </div>
              <div class="pass-meta-item">
                <span class="meta-field-label">RETURN DATE</span>
                <span class="meta-field-val">
                  {{ wizardForm.end_at ? new Date(wizardForm.end_at).toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }}
                </span>
              </div>
              <div class="pass-meta-item">
                <span class="meta-field-label">TRIP BUDGET</span>
                <span class="meta-field-val font-bold text-emerald-400">
                  {{ formatINR((wizardForm.estimated_budget || 0) * 100) }}
                </span>
              </div>
              <div class="pass-meta-item">
                <span class="meta-field-label">LEAD / MUNIMJI</span>
                <span class="meta-field-val">{{ authStore.user?.full_name || 'You' }}</span>
              </div>
            </div>

            <!-- Barcode & Ticket Footer -->
            <div class="pass-barcode-footer">
              <div class="barcode-graphic">
                <span class="bar b1"></span><span class="bar b3"></span><span class="bar b2"></span><span class="bar b4"></span>
                <span class="bar b1"></span><span class="bar b2"></span><span class="bar b3"></span><span class="bar b1"></span>
                <span class="bar b4"></span><span class="bar b2"></span><span class="bar b1"></span><span class="bar b3"></span>
                <span class="bar b2"></span><span class="bar b4"></span><span class="bar b1"></span><span class="bar b2"></span>
              </div>
              <span class="pass-serial-num">SPL-{{ wizardForm.title.trim() ? wizardForm.title.trim().replace(/\s+/g, '-').slice(0, 10).toUpperCase() : 'TRIP' }}-{{ new Date().getFullYear() }}</span>
            </div>
          </div>

          <!-- Bottom helpful tip -->
          <div class="ticket-bottom-tip">
            <i class="pi pi-shield text-emerald-400"></i>
            <span>This boarding ticket activates instantly once created.</span>
          </div>
        </aside>

        <!-- Right 70%: Wizard Form Surface Card -->
        <section class="wizard-surface-card">
          <!-- Wizard Header & Breadcrumb / Cancel -->
          <div class="wizard-top-nav">
            <div class="wizard-branding">
              <span class="wizard-badge">TRIP SETUP</span>
              <h1 class="wizard-headline brand-font">{{ wizardHeading }}</h1>
            </div>
            <Button 
              v-if="wizardStep !== 4"
              @click="cancelCreateTripWizard" 
              icon="pi pi-times" 
              label="Cancel" 
              size="small" 
              text 
              severity="secondary" 
              class="wizard-cancel-btn"
            />
          </div>

          <!-- Wizard Stepper Indicator -->
          <div class="wizard-stepper" v-if="wizardStep !== 4">
            <div 
              class="stepper-node" 
              :class="{ 
                'node-active': wizardStep === 1, 
                'node-completed': wizardStep > 1 
              }"
              @click="goToWizardStep(1)"
            >
              <div class="node-circle">
                <i v-if="wizardStep > 1" class="pi pi-check"></i>
                <span v-else>1</span>
              </div>
              <div class="node-meta">
                <span class="node-step-label">STEP 1</span>
                <span class="node-title">Trip Name</span>
              </div>
            </div>

            <div class="stepper-connector" :class="{ 'connector-completed': wizardStep > 1 }"></div>

            <div 
              class="stepper-node" 
              :class="{ 
                'node-active': wizardStep === 2, 
                'node-completed': wizardStep > 2,
                'node-disabled': !isStep1Valid
              }"
              @click="goToWizardStep(2)"
            >
              <div class="node-circle">
                <i v-if="wizardStep > 2" class="pi pi-check"></i>
                <span v-else>2</span>
              </div>
              <div class="node-meta">
                <span class="node-step-label">STEP 2</span>
                <span class="node-title">Route & Dates</span>
              </div>
            </div>

            <div class="stepper-connector" :class="{ 'connector-completed': wizardStep > 2 }"></div>

            <div 
              class="stepper-node" 
              :class="{ 
                'node-active': wizardStep === 3, 
                'node-completed': wizardStep > 3,
                'node-disabled': !isStep1Valid || !isStep2Valid
              }"
              @click="goToWizardStep(3)"
            >
              <div class="node-circle">
                <i v-if="wizardStep > 3" class="pi pi-check"></i>
                <span v-else>3</span>
              </div>
              <div class="node-meta">
                <span class="node-step-label">STEP 3</span>
                <span class="node-title">Estimated Budget</span>
              </div>
            </div>
          </div>

          <!-- STEP 1: Trip Name -->
          <div class="wizard-step-body" v-if="wizardStep === 1">
            <div class="step-intro">
              <div class="step-icon-badge">
                <i class="pi pi-compass"></i>
              </div>
              <div>
                <h2 class="step-heading brand-font">What should be the name of the trip?</h2>
                <p class="step-subheading">
                  Give your journey a clear name to identify the shared group ledger and expenses.
                </p>
              </div>
            </div>

            <div class="wizard-fields-cluster">
              <div class="form-control">
                <label class="form-label font-bold">Trip Name</label>
                <InputText 
                  v-model="wizardForm.title" 
                  placeholder="e.g. Kasol Trail 2026, Goa Friends Trip, Konkan Weekend" 
                  autofocus
                  class="wizard-neu-input wizard-title-input" 
                  @keyup.enter="nextWizardStep"
                />
                <span class="input-hint-subtle">
                  <i class="pi pi-info-circle"></i> Minimum 3 characters.
                </span>
              </div>
            </div>

            <div class="wizard-actions-bar">
              <Button 
                @click="cancelCreateTripWizard" 
                label="Cancel" 
                severity="secondary" 
                text 
                size="small"
              />
              <Button 
                @click="nextWizardStep" 
                :disabled="!isStep1Valid" 
                label="Next: Route & Dates" 
                icon="pi pi-arrow-right" 
                iconPos="right" 
                severity="primary" 
                size="large"
                class="btn-wizard-next"
              />
            </div>
          </div>

          <!-- STEP 2: Route & Dates -->
          <div class="wizard-step-body" v-if="wizardStep === 2">
            <div class="step-intro">
              <div class="step-icon-badge">
                <i class="pi pi-map"></i>
              </div>
              <div>
                <h2 class="step-heading brand-font">Where does the trip start, and what is the destination?</h2>
                <p class="step-subheading">
                  Specify your starting location, destination, and the travel dates.
                </p>
              </div>
            </div>

            <div class="wizard-fields-cluster">
              <!-- 1. TRAVEL DATES & TIME (TOP) -->
              <div class="wizard-section-box">
                <div class="wizard-section-title">
                  <i class="pi pi-calendar text-[var(--accent)] mr-1"></i> Travel Timing & Duration
                </div>
                <div class="wizard-two-col">
                  <div class="form-control">
                    <label class="form-label font-bold">
                      <i class="pi pi-calendar-plus text-emerald-400 mr-1"></i> Start Date & Time
                    </label>
                    <InputText 
                      v-model="wizardForm.start_at" 
                      type="datetime-local" 
                      class="wizard-neu-input" 
                      :class="{ 'input-has-error': dateValidationError }"
                    />
                    <span class="input-hint-subtle">When does the journey commence?</span>
                  </div>

                  <div class="form-control">
                    <label class="form-label font-bold">
                      <i class="pi pi-calendar-times text-rose-400 mr-1"></i> End Date & Time
                    </label>
                    <InputText 
                      v-model="wizardForm.end_at" 
                      type="datetime-local" 
                      :min="wizardForm.start_at"
                      class="wizard-neu-input" 
                      :class="{ 'input-has-error': dateValidationError }"
                    />
                    <span class="input-hint-subtle">When do you expect to conclude?</span>
                  </div>
                </div>

                <!-- Instant date validation alert -->
                <div v-if="dateValidationError" class="date-validation-error-banner">
                  <i class="pi pi-exclamation-triangle"></i>
                  <span>{{ dateValidationError }}</span>
                </div>
                <!-- Live duration & 24h timeline clarification -->
                <div v-else-if="tripDurationSummary" class="trip-duration-badge">
                  <i class="pi pi-clock text-[var(--accent)] mr-1.5"></i>
                  <span>{{ tripDurationSummary }}</span>
                </div>
              </div>

              <!-- 2. EXPEDITION ROUTE & STOPS (MIDDLE) -->
              <div class="wizard-section-box route-expedition-box">
                <div class="route-section-heading">
                  <div class="route-section-title">
                    <i class="pi pi-compass text-[var(--accent)] mr-2"></i> Expedition Route & Halts
                  </div>
                  <span class="route-section-subtitle">Define your departure origin, arrange stops in travel sequence, and set destination.</span>
                </div>

                <!-- Starting Point (Prominent Milestone Card) -->
                <div class="route-milestone-card start-milestone-card">
                  <div class="milestone-badge-row">
                    <span class="milestone-pill start-pill">
                      <i class="pi pi-map-marker mr-1.5"></i> STARTING POINT
                    </span>
                    <span class="milestone-role-hint">Origin / Departure City</span>
                  </div>
                  <LocationAutocomplete
                    v-model="wizardForm.start_point"
                    ariaLabel="Starting point"
                    placeholder="Search departure city, landmark or station"
                    @select="wizardForm.start_place = $event"
                  />
                  <div class="milestone-subtext">
                    <i class="pi pi-check-circle mr-1 text-emerald-400"></i> Select a suggested place to pin the exact GPS departure point.
                  </div>
                </div>

                <!-- Intermediate Stops with Drag & Drop Shuffle -->
                <div class="trip-stops-editor">
                  <div class="trip-stops-heading">
                    <div>
                      <span class="stops-section-title">
                        <i class="pi pi-directions text-[var(--accent)] mr-1.5"></i> Intermediate Stops & Halts
                      </span>
                      <span class="input-hint-subtle">Drag handle <i class="pi pi-bars text-xs"></i> to shuffle stops in travel order.</span>
                    </div>
                    <Button
                      type="button"
                      label="Add stop"
                      icon="pi pi-plus"
                      size="small"
                      severity="secondary"
                      outlined
                      :disabled="wizardForm.stops.length >= 20"
                      @click="addTripStop"
                    />
                  </div>

                  <div v-if="!wizardForm.stops.length" class="trip-stops-empty-hint">
                    <i class="pi pi-compass mr-1 text-[var(--accent)]"></i> No intermediate halts added yet. Click <strong>+ Add stop</strong> if you plan to make waypoints.
                  </div>

                  <div class="trip-stops-list" v-if="wizardForm.stops.length">
                    <div
                      v-for="(stop, index) in wizardForm.stops"
                      :key="stop.id"
                      :data-index="index"
                      class="trip-stop-row"
                      :class="{
                        'is-dragging': draggedStopIndex === index,
                        'is-drag-over': dragOverStopIndex === index && draggedStopIndex !== index
                      }"
                      draggable="true"
                      @dragstart="onStopDragStart(index, $event)"
                      @dragover="onStopDragOver(index, $event)"
                      @drop="onStopDrop(index)"
                      @dragend="onStopDragEnd"
                    >
                      <!-- Drag handle with mobile touch support -->
                      <div 
                        class="trip-stop-drag-handle" 
                        title="Drag to shuffle stop order"
                        @touchstart="onTouchDragStart(index)"
                        @touchmove="onTouchDragMove"
                        @touchend="onTouchDragEnd"
                        @touchcancel="onTouchDragEnd"
                      >
                        <i class="pi pi-bars"></i>
                      </div>

                      <!-- Stop Number Badge (Cyan Blue) -->
                      <span class="trip-stop-order" :title="`Stop ${index + 1}`">
                        {{ index + 1 }}
                      </span>

                      <!-- Location Search -->
                      <div class="trip-stop-input-wrap">
                        <LocationAutocomplete
                          v-model="stop.input"
                          :ariaLabel="`Stop ${index + 1}`"
                          placeholder="Search waypoint or halt"
                          @select="stop.place = $event"
                        />
                      </div>

                      <!-- Delete control -->
                      <div class="trip-stop-actions">
                        <Button
                          type="button"
                          icon="pi pi-trash"
                          severity="danger"
                          text
                          rounded
                          class="trip-stop-delete-btn"
                          :aria-label="`Remove stop ${index + 1}`"
                          @click="removeTripStop(index)"
                        />
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Destination (Prominent Milestone Card) -->
                <div class="route-milestone-card destination-milestone-card">
                  <div class="milestone-badge-row">
                    <span class="milestone-pill dest-pill">
                      <i class="pi pi-flag mr-1.5"></i> FINAL DESTINATION
                    </span>
                    <span class="milestone-role-hint">Arrival / Concluding City</span>
                  </div>
                  <LocationAutocomplete
                    v-model="wizardForm.destination"
                    ariaLabel="Destination"
                    placeholder="Search arrival city or final landmark"
                    @select="wizardForm.destination_place = $event"
                  />
                  <div class="milestone-subtext">
                    <i class="pi pi-check-circle mr-1 text-rose-400"></i> Select a suggested place to pin the destination on the map.
                  </div>
                </div>
              </div>

              <!-- 3. TOTAL TRAVELLING DISTANCE (EXACTLY BELOW LOCATION STOPS) -->
              <div v-if="routeMapPoints.length >= 2" class="total-distance-card">
                <div class="total-distance-header">
                  <div class="total-distance-badge-wrap">
                    <div class="total-distance-icon-glow">
                      <i class="pi pi-car"></i>
                    </div>
                    <div>
                      <div class="total-distance-label">TOTAL TRAVELLING DISTANCE</div>
                      <div class="total-distance-value" v-if="!routeDistanceInfo?.isLoading && routeDistanceInfo?.totalDistance">
                        {{ formatDistanceKm(routeDistanceInfo.totalDistance) }}
                      </div>
                      <div class="total-distance-loading" v-else-if="routeDistanceInfo?.isLoading">
                        <i class="pi pi-spin pi-spinner mr-1 text-[var(--accent)]"></i> Calculating driving route distance…
                      </div>
                      <div class="total-distance-direct" v-else-if="routeDistanceInfo?.error">
                        <i class="pi pi-info-circle mr-1"></i> Direct-line preview (road routing unavailable)
                      </div>
                    </div>
                  </div>

                  <div v-if="routeDistanceInfo?.legs?.length" class="legs-count-pill">
                    <i class="pi pi-map mr-1"></i> {{ routeDistanceInfo.legs.length }} {{ routeDistanceInfo.legs.length === 1 ? 'Leg' : 'Legs' }}
                  </div>
                </div>

                <!-- Distance Legs Breakdown -->
                <div v-if="routeDistanceInfo?.legs?.length" class="total-distance-legs-list">
                  <div v-for="(leg, idx) in routeDistanceInfo.legs" :key="idx" class="distance-leg-item">
                    <div class="leg-left">
                      <span class="leg-index">{{ idx + 1 }}</span>
                      <span class="leg-names">
                        <strong>{{ leg.from }}</strong>
                        <i class="pi pi-arrow-right leg-arrow"></i>
                        <strong>{{ leg.to }}</strong>
                      </span>
                    </div>
                    <span class="leg-km">{{ formatDistanceKm(leg.distance) }}</span>
                  </div>
                </div>
              </div>

              <!-- 4. LIVE ROUTE MAP (EKDUM BELOW) -->
              <div class="route-map-preview">
                <div class="route-map-preview-heading">
                  <div>
                    <span class="form-label font-bold">Interactive Route Map</span>
                    <span class="input-hint-subtle">Driving route with your selected waypoints & stops.</span>
                  </div>
                  <span class="route-distance-note">OpenStreetMap + OSRM Driving Engine</span>
                </div>
                <RouteMap 
                  v-if="routeMapPoints.length >= 2" 
                  :points="routeMapPoints" 
                  :hide-distance-summary="true"
                  @route-calculated="onRouteCalculated"
                />
                <p v-else class="route-map-empty">
                  Choose a suggested start, destination, and all added stops to preview the route and calculate distance.
                </p>
              </div>
            </div>

            <div class="wizard-actions-bar">
              <Button 
                @click="prevWizardStep" 
                label="Back" 
                icon="pi pi-arrow-left" 
                severity="secondary" 
                text 
                size="small"
                class="btn-wizard-back-small"
              />
              <Button 
                @click="nextWizardStep" 
                :disabled="!isStep2Valid" 
                label="Next: Estimated Budget" 
                icon="pi pi-arrow-right" 
                iconPos="right" 
                severity="primary" 
                size="large"
                class="btn-wizard-next"
              />
            </div>
          </div>

          <!-- STEP 3: Estimated Budget -->
          <div class="wizard-step-body" v-if="wizardStep === 3">
            <div class="step-intro">
              <div class="step-icon-badge">
                <i class="pi pi-wallet"></i>
              </div>
              <div>
                <h2 class="step-heading brand-font">Estimated Budget of the Trip</h2>
                <p class="step-subheading">
                  Set the expected overall budget for this group trip.
                </p>
              </div>
            </div>

            <div class="wizard-fields-cluster">
              <div class="form-control">
                <label class="form-label font-bold">
                  <i class="pi pi-wallet mr-1 text-emerald-400"></i> Estimated Trip Budget (₹)
                </label>
                <InputNumber 
                  v-model="wizardForm.estimated_budget" 
                  :min="1" 
                  mode="currency" 
                  currency="INR" 
                  locale="en-IN" 
                  placeholder="Enter estimated group budget" 
                  class="wizard-neu-input" 
                  fluid
                />
                <span class="input-hint-subtle">
                  Expected total expenditure of the trip (saved in database as estimated_budget).
                </span>
              </div>

              <!-- Blueprint Summary -->
              <div class="wizard-summary-card">
                <div class="summary-card-header">
                  <span class="summary-card-title brand-font">Trip Summary</span>
                  <Tag severity="info" value="Munimji Access" rounded />
                </div>
                <div class="summary-details-grid">
                  <div class="summary-col">
                    <span class="summary-dim">TRIP</span>
                    <span class="summary-val font-bold">{{ wizardForm.title }}</span>
                  </div>
                  <div class="summary-col">
                    <span class="summary-dim">ROUTE</span>
                    <span class="summary-val">{{ wizardForm.start_point }} → {{ wizardForm.destination }}</span>
                  </div>
                  <div class="summary-col">
                    <span class="summary-dim">ESTIMATED BUDGET</span>
                    <span class="summary-val font-bold text-emerald-400">
                      {{ formatINR((wizardForm.estimated_budget || 0) * 100) }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="wizard-actions-bar">
              <Button 
                @click="prevWizardStep" 
                label="Back" 
                icon="pi pi-arrow-left" 
                severity="secondary" 
                text 
                size="small"
                class="btn-wizard-back-small"
              />
              <Button 
                @click="handleWizardSubmit" 
                :loading="creatingTripLoading"
                :disabled="!isStep3Valid" 
                label="Create the Trip" 
                icon="pi pi-check" 
                severity="primary" 
                size="large"
                class="btn-wizard-launch"
              />
            </div>
          </div>

          <!-- STEP 4: Success & Celebration Deck -->
          <div class="wizard-step-body wizard-success-deck" v-if="wizardStep === 4 && createdTripResult">
            <div class="success-icon-badge">
              <i class="pi pi-check"></i>
            </div>

            <span class="success-kicker">TRIP CREATED SUCCESSFULLY</span>
            <h2 class="success-headline brand-font">Congratulations!</h2>
            <h3 class="success-trip-title brand-font">{{ createdTripResult.title || createdTripResult.name }}</h3>

            <p class="success-description">
              Your trip <strong>{{ createdTripResult.title || createdTripResult.name }}</strong> has been successfully created. 
              You can now start adding participants and log the expenses there!
            </p>

            <div class="success-manifest-strip">
              <div class="manifest-pill">
                <i class="pi pi-map-marker text-emerald-400"></i>
                <span>{{ createdTripResult.start_point || createdTripResult.start_location }} → {{ createdTripResult.destination }}</span>
              </div>
              <div class="manifest-pill">
                <i class="pi pi-crown text-amber-400"></i>
                <span>Role: Munimji</span>
              </div>
              <div class="manifest-pill">
                <i class="pi pi-wallet text-sky-400"></i>
                <span>Budget: {{ formatINR(Math.round(((createdTripResult.estimated_budget !== undefined ? createdTripResult.estimated_budget : 0) * 100) || (createdTripResult.budget_paise || 0))) }}</span>
              </div>
            </div>

            <div class="success-actions-row">
              <Button 
                @click="handleRedirectToCreatedTrip" 
                label="Redirect to Trip" 
                icon="pi pi-arrow-right" 
                iconPos="right" 
                severity="primary" 
                size="large"
                class="btn-enter-workspace"
              />
              <Button 
                @click="openCreateTripWizard" 
                label="Plan Another Trip" 
                icon="pi pi-plus" 
                severity="secondary" 
                text 
                size="large"
              />
            </div>
          </div>
        </section>
      </main>

      <!-- Center & Right: Selected Trip Ledger -->
      <main class="ledger-column" v-else-if="selectedTrip">
        <!-- Trip Context Banner -->
        <section class="trip-hero-section">
          <div class="hero-left">
            <span class="hero-badge">EXPEDITION MANIFEST</span>
            <h1 class="hero-title brand-font">{{ selectedTrip.title }}</h1>
            <div class="hero-route-line">
              <span class="route-node">{{ selectedTrip.start_point }}</span>
              <template v-for="stop in selectedTrip.stops || []" :key="stop.id || stop.position">
                <i class="pi pi-arrow-right route-arrow-icon"></i>
                <span class="route-node">{{ stop.name }}</span>
              </template>
              <i class="pi pi-arrow-right route-arrow-icon"></i>
              <span class="route-node">{{ selectedTrip.destination }}</span>
            </div>

          </div>

          <div class="hero-right">
            <!-- Expedition Status & Actions Bar -->
            <div class="hero-actions-cluster">
              <div class="status-control-box" v-if="isMunimji">
                <span class="chip-role">EXPEDITION STATUS</span>
                <Select
                  :modelValue="selectedTrip.status"
                  @update:modelValue="handleUpdateTripStatus($event)"
                  :options="statusOptions"
                  optionLabel="label"
                  optionValue="value"
                  class="status-select-component"
                  size="small"
                >
                  <template #value="slotProps">
                    <div v-if="slotProps.value" class="flex items-center gap-1.5 text-xs font-bold text-[var(--text-main)]">
                      <i :class="statusOptions.find(o => o.value === slotProps.value)?.icon || 'pi pi-circle'" class="text-xs"></i>
                      <span>{{ statusOptions.find(o => o.value === slotProps.value)?.label }}</span>
                    </div>
                  </template>
                  <template #option="slotProps">
                    <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                      <i :class="slotProps.option.icon" class="text-xs"></i>
                      <span>{{ slotProps.option.label }}</span>
                    </div>
                  </template>
                </Select>
              </div>
              <div class="status-control-box" v-else>
                <span class="chip-role">EXPEDITION STATUS</span>
                <span class="status-pill status-readonly">{{ selectedTrip.status.toUpperCase() }}</span>
              </div>

              <!-- Delete Expedition Button (Munimji Only) -->
              <Button
                v-if="isMunimji && ['planned', 'active'].includes(selectedTrip.status)"
                @click="openEditTripWizard"
                icon="pi pi-pencil"
                label="Edit Route"
                size="small"
                severity="secondary"
                outlined
                :loading="creatingTripLoading"
                class="btn-edit-expedition"
              />
              <Button
                v-if="isMunimji"
                @click="showDeleteTripModal = true"
                icon="pi pi-trash"
                label="Delete Trip"
                size="small"
                severity="danger"
                outlined
                class="btn-delete-expedition"
                title="Permanently delete this expedition and all records"
              />
            </div>

            <div class="treasurer-chip">
              <span class="chip-role">YOUR ROLE</span>
              <span class="chip-name brand-font">
                {{ selectedTrip.user_role === 'munimji' ? '👑 Munimji' : selectedTrip.user_role === 'co_reviewer' ? '⚖️ Co-Reviewer' : '👤 Participant' }}
              </span>
            </div>
          </div>
        </section>

        <!-- If user is invited and not yet active, show dedicated Invitation Decision Hero Deck -->
        <section v-if="selectedTrip.user_status === 'invited'" class="invited-decision-deck">
          <div class="invited-decision-card">
            <div class="invited-icon-badge">
              <i class="pi pi-envelope"></i>
            </div>
            <span class="invited-pill">EXPEDITION INVITATION PENDING</span>
            <h2 class="invited-title brand-font">You're Invited to Join {{ selectedTrip.title }}</h2>
            <p class="invited-desc">
              Munimji has invited you to join this expedition as 
              <strong>{{ selectedTrip.user_role === 'co_reviewer' ? 'Co-Reviewer (Auditor)' : 'Participant' }}</strong>.
              Accept the invitation below to unlock the treasury ledger, submit claims, and participate in peer settlement.
            </p>

            <div class="invited-trip-summary">
              <div class="summary-item">
                <span class="summary-label">Route</span>
                <span class="summary-val font-semibold">{{ selectedTrip.start_point }} → {{ selectedTrip.destination }}</span>
              </div>
              <div class="summary-item" v-if="selectedTrip.start_at && selectedTrip.end_at">
                <span class="summary-label">Dates</span>
                <span class="summary-val font-semibold">{{ new Date(selectedTrip.start_at).toLocaleDateString() }} - {{ new Date(selectedTrip.end_at).toLocaleDateString() }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Assigned Role</span>
                <span class="summary-val role-highlight">{{ selectedTrip.user_role === 'co_reviewer' ? 'Co-Reviewer (Auditor)' : 'Participant' }}</span>
              </div>
            </div>

            <div class="invited-actions-row">
              <Button 
                @click="handleRespondInvitation(selectedTrip.id, 'accept')" 
                :loading="respondingInvitation" 
                label="Accept Invitation & Join Expedition" 
                icon="pi pi-check" 
                severity="success" 
                size="large"
              />
              <Button 
                @click="handleRespondInvitation(selectedTrip.id, 'reject')" 
                :loading="respondingInvitation" 
                label="Decline Invitation" 
                icon="pi pi-times" 
                severity="danger" 
                outlined 
                size="large"
              />
            </div>
            <p class="invited-fineprint">
              By accepting, your account will be activated in the expedition manifest and you will be able to submit expense claims and view ledger balances.
            </p>
          </div>
        </section>

        <!-- Otherwise, if active member, render workspace tabs -->
        <template v-else>
          <!-- Modern Mobile & Desktop Responsive Tab Bar -->
          <nav class="workspace-tabs-bar">
            <button 
              type="button"
              @click="activeTab = 'ledger'" 
              :class="['workspace-tab-btn', { active: activeTab === 'ledger' }]"
            >
              <i class="pi pi-chart-bar"></i>
              <span>Treasury & Ledger</span>
            </button>
            <button 
              type="button"
              @click="activeTab = 'claims'" 
              :class="['workspace-tab-btn', { active: activeTab === 'claims' }]"
            >
              <i class="pi pi-receipt"></i>
              <span>Claims</span>
              <span v-if="claimsList.length > 0" class="nav-count-pill">{{ claimsList.length }}</span>
            </button>
            <button 
              type="button"
              @click="activeTab = 'members'" 
              :class="['workspace-tab-btn', { active: activeTab === 'members' }]"
            >
              <i class="pi pi-users"></i>
              <span>Members</span>
              <span v-if="membersList.length > 0" class="nav-count-pill">{{ membersList.length }}</span>
            </button>
            <button 
              type="button"
              @click="activeTab = 'settlement'" 
              :class="['workspace-tab-btn', { active: activeTab === 'settlement' }]"
            >
              <i class="pi pi-arrows-h"></i>
              <span>Settlement</span>
              <span v-if="settlementData?.transfers?.length > 0" class="nav-count-pill">{{ settlementData.transfers.length }}</span>
            </button>
          </nav>

          <!-- TAB 1: Ledger & Treasury -->
          <div v-if="activeTab === 'ledger'">


          <!-- Financial Metrics Grid -->
          <section v-if="dashboard" class="metrics-grid">
            <div class="metric-card">
              <span class="metric-heading">TOTAL CONTRIBUTIONS</span>
              <div class="metric-number mono-num">{{ formatINR(dashboard.total_contributions_paise) }}</div>
              <span class="metric-sub">Confirmed pool receipts</span>
            </div>

            <div class="metric-card">
              <span class="metric-heading">GROUP-FUNDED SPENT</span>
              <div class="metric-number mono-num">{{ formatINR(dashboard.group_funded_spend_paise) }}</div>
              <span class="metric-sub">Tolls, hotel, fuel & common food</span>
            </div>

            <div class="metric-card">
              <span class="metric-heading">AVAILABLE CASH IN HAND</span>
              <div class="metric-number mono-num">{{ formatINR(dashboard.available_cash_paise) }}</div>
              <span class="metric-sub">Central treasury with Munimji</span>
            </div>

            <div class="metric-card">
              <span class="metric-heading">APPROVED PEER CLAIMS</span>
              <div class="metric-number mono-num">{{ formatINR(dashboard.approved_claims_paise) }}</div>
              <span class="metric-sub">Out-of-pocket pending payout</span>
            </div>
          </section>

          <!-- Operations Action Deck -->
          <section class="operations-deck">
            <div class="deck-header">
              <h3 class="deck-title brand-font">Operations & Records</h3>
              <p class="deck-caption">Record transactions with immutable ledger traceability.</p>
            </div>

            <div class="action-grid">
              <div class="action-tile">
                <div class="tile-meta">
                  <span class="tile-category">PEER REIMBURSEMENT</span>
                  <h4 class="tile-title brand-font">File Expense Claim</h4>
                </div>
                <p class="tile-text">Paid out-of-pocket for petrol or meals? Submit claim for Munimji audit.</p>
                <Button @click="openClaimModal()" label="Submit Claim" icon="pi pi-plus" size="small" outlined fluid />
              </div>

              <div class="action-tile">
                <div class="tile-meta">
                  <span class="tile-category">TREASURER ENTRY</span>
                  <h4 class="tile-title brand-font">Record Member Inflow</h4>
                </div>
                <p class="tile-text">Acknowledge contribution payment received from participant into group funds.</p>
                <Button @click="openContributionModal()" label="Log Receipt" icon="pi pi-arrow-down-left" size="small" outlined fluid />
              </div>

              <div class="action-tile">
                <div class="tile-meta">
                  <span class="tile-category">GROUP OUTFLOW</span>
                  <h4 class="tile-title brand-font">Record Group Expense</h4>
                </div>
                <p class="tile-text">Paid hotel, tolls, or common food directly from collected cash/UPI pool?</p>
                <Button @click="openExpenseModal()" label="Log Expense" icon="pi pi-arrow-up-right" size="small" outlined fluid />
              </div>

              <div class="action-tile">
                <div class="tile-meta">
                  <span class="tile-category">SETTLEMENT AUDIT</span>
                  <h4 class="tile-title brand-font">Issue PDF Statement</h4>
                </div>
                <p class="tile-text">Compile cryptographic ledger snapshot and download PDF settlement statement.</p>
                <Button 
                  @click="handleGenerateReport" 
                  :loading="generatingReport"
                  label="Generate Report" 
                  icon="pi pi-file-pdf" 
                  size="small" 
                  severity="primary" 
                  fluid
                />
              </div>
            </div>
          </section>

          <!-- Ledger History Section: Contributions & Expenses with Timestamps -->
          <section class="ledger-records-section">
            <div class="records-header-row">
              <div class="records-title-group">
                <h3 class="deck-title brand-font">Transaction Ledger</h3>
                <p class="deck-caption">Itemized record of participant contributions and common pool outflows.</p>
              </div>
              <div class="records-subnav-pills">
                <button 
                  @click="ledgerSubTab = 'contributions'" 
                  :class="['subnav-pill', { active: ledgerSubTab === 'contributions' }]"
                >
                  <span>💰 Contributions ({{ contributionsList.length }})</span>
                </button>
                <button 
                  @click="ledgerSubTab = 'expenses'" 
                  :class="['subnav-pill', { active: ledgerSubTab === 'expenses' }]"
                >
                  <span>🧾 Group Outflows ({{ expensesList.length }})</span>
                </button>
              </div>
            </div>

            <!-- List of Member Contributions -->
            <div v-if="ledgerSubTab === 'contributions'" class="records-content-container">
              <div v-if="contributionsList.length === 0" class="empty-records-msg">
                No contributions logged for this trip yet. Click "+ Log Receipt" to add one.
              </div>
              <div v-else class="record-table-wrapper">
                <table class="neu-table">
                  <thead>
                    <tr>
                      <th>PARTICIPANT</th>
                      <th>PAYMENT METHOD</th>
                      <th>DATE & TIME</th>
                      <th>REFERENCE / NOTE</th>
                      <th class="text-right">AMOUNT</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="c in contributionsList" :key="c.id">
                      <td class="font-bold brand-font">{{ c.user_name || 'Member' }}</td>
                      <td>
                        <span class="method-tag">{{ c.payment_method?.toUpperCase() }}</span>
                      </td>
                      <td class="mono-num text-muted">{{ new Date(c.received_at).toLocaleString() }}</td>
                      <td class="text-muted">{{ c.notes || '—' }}</td>
                      <td class="mono-num text-right amount-inflow">+{{ formatINR(c.amount_paise) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- List of Group Expenses -->
            <div v-if="ledgerSubTab === 'expenses'" class="records-content-container">
              <div v-if="expensesList.length === 0" class="empty-records-msg">
                No group expenses recorded yet. Click "+ Log Expense" to add one.
              </div>
              <div v-else class="record-table-wrapper">
                <table class="neu-table">
                  <thead>
                    <tr>
                      <th>DESCRIPTION</th>
                      <th>CATEGORY</th>
                      <th>RECORDED DATE</th>
                      <th>PAID BY</th>
                      <th class="text-right">AMOUNT</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="e in expensesList" :key="e.id">
                      <td class="font-bold brand-font">{{ e.title }}</td>
                      <td>
                        <span class="category-badge">{{ e.category }}</span>
                      </td>
                      <td class="mono-num text-muted">{{ new Date(e.expense_date).toLocaleString() }}</td>
                      <td class="text-muted">{{ e.paid_by_name || 'Munimji' }}</td>
                      <td class="mono-num text-right amount-outflow">-{{ formatINR(e.amount_paise) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </section>
        </div>

        <!-- TAB 2: Peer Claims Review Protocol -->
        <section v-if="activeTab === 'claims'" class="claims-review-deck">
          <div class="deck-header-row">
            <div>
              <h3 class="deck-title brand-font">Peer Claims & Review Protocol</h3>
              <p class="deck-caption">Out-of-pocket expenses awaiting Munimji or Co-Reviewer approval.</p>
            </div>
            <Button 
              @click="openClaimModal()" 
              label="File New Claim" 
              icon="pi pi-plus" 
              size="small" 
              rounded 
            />
          </div>

          <div v-if="claimsList.length === 0" class="empty-claims-msg">
            No claims filed for this trip yet.
          </div>

          <div v-else class="claims-list-stack">
            <div 
              v-for="claim in claimsList" 
              :key="claim.id"
              class="claim-neu-card"
            >
              <div class="claim-card-left">
                <div class="claim-user-tag">
                  <span class="claimant-name brand-font">{{ claim.claimant_name || 'Member' }}</span>
                  <Tag 
                    :value="claim.status.toUpperCase()" 
                    :severity="claim.status === 'approved' ? 'success' : claim.status === 'rejected' ? 'danger' : 'warn'" 
                    rounded 
                  />
                </div>
                <div class="claim-reason">{{ claim.reason }}</div>
                <div class="claim-meta-details">
                  <span class="category-badge">{{ claim.category }}</span>
                  <span class="claim-date">{{ new Date(claim.expense_date).toLocaleDateString() }}</span>
                  <button 
                    v-if="claim.receipt_path || claim.receipt_url" 
                    @click.stop="openReceiptViewer(claim)" 
                    class="receipt-view-link mono-num"
                    type="button"
                    title="View Attached Receipt"
                  >
                    <i class="pi pi-receipt text-xs"></i> View Receipt
                  </button>
                </div>
              </div>

              <div class="claim-card-right">
                <div class="claim-amount mono-num">{{ formatINR(claim.amount_paise) }}</div>
                <div v-if="canReviewClaim(claim)" class="claim-action-buttons">
                  <Button 
                    @click="handleDecideClaim(claim.id, true)" 
                    label="Approve" 
                    icon="pi pi-check" 
                    severity="success" 
                    size="small" 
                  />
                  <Button 
                    @click="handleDecideClaim(claim.id, false)" 
                    label="Reject" 
                    icon="pi pi-times" 
                    severity="danger" 
                    size="small" 
                    outlined 
                  />
                </div>
                <div v-else-if="claim.status === 'pending' && claim.claimant_id === authStore.user?.id" class="pending-awaiting-note">
                  <Tag value="Awaiting Review" severity="warn" icon="pi pi-clock" rounded />
                </div>
                <div v-else-if="claim.status === 'pending'" class="pending-awaiting-note">
                  <Tag value="Pending Decision" severity="secondary" rounded />
                </div>
                <div v-else-if="claim.status === 'rejected' && claim.rejection_reason" class="rejection-note">
                  Reason: {{ claim.rejection_reason }}
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- TAB 3: Trip Members & Governance Deck -->
        <section v-if="activeTab === 'members'" class="members-governance-deck">
          <div class="deck-header-row">
            <div>
              <h3 class="deck-title brand-font">Trip Participants & Governance</h3>
              <p class="deck-caption">Active expedition members and invitation audit trail.</p>
            </div>
            <!-- Munimji is the only one who can invite peers -->
            <Button 
              v-if="isMunimji" 
              @click="openInviteModal" 
              label="Invite Peer" 
              icon="pi pi-user-plus" 
              size="small" 
              rounded 
            />
          </div>

          <!-- Section 1: Active Participants -->
          <div class="members-sub-section">
            <h4 class="sub-deck-title brand-font">👥 Active Expedition Members ({{ membersList.length }})</h4>
            <div v-if="membersList.length === 0" class="empty-records-msg">
              No active participants yet.
            </div>
            <div v-else class="members-grid-stack">
              <div 
                v-for="member in membersList" 
                :key="member.id"
                class="member-neu-card"
              >
                <div class="member-info">
                  <div class="member-header">
                    <span class="member-name brand-font">{{ member.user_name || member.email?.split('@')[0] || 'Member' }}</span>
                    <div class="member-badges-row">
                      <Tag 
                        :value="member.role === 'munimji' ? 'Munimji' : member.role === 'co_reviewer' ? 'Co-Reviewer' : 'Participant'" 
                        :severity="member.role === 'munimji' ? 'warn' : member.role === 'co_reviewer' ? 'info' : 'secondary'"
                        :icon="member.role === 'munimji' ? 'pi pi-crown' : member.role === 'co_reviewer' ? 'pi pi-shield' : 'pi pi-user'"
                        rounded
                      />
                      <Tag value="Active" severity="success" icon="pi pi-check" rounded />
                    </div>
                  </div>
                  <span class="member-email">{{ member.email }}</span>
                </div>

                <div class="member-card-footer">
                  <span class="member-join-date mono-num">
                    {{ member.joined_at ? `Joined ${new Date(member.joined_at).toLocaleDateString()}` : 'Active Member' }}
                  </span>

                  <!-- Munimji Actions: Configure Role & Remove -->
                  <div class="member-actions-row" v-if="isMunimji && member.role !== 'munimji' && member.user_id !== authStore.user?.id">
                    <!-- Co-Reviewer Role Configuration Toggle -->
                    <Button 
                      v-if="member.role === 'participant'"
                      @click="handleUpdateRole(member, 'co_reviewer')" 
                      :loading="updatingRoleId === member.user_id"
                      label="Make Co-Reviewer" 
                      icon="pi pi-shield" 
                      severity="info" 
                      size="small" 
                      outlined 
                    />
                    <Button 
                      v-else-if="member.role === 'co_reviewer'"
                      @click="handleUpdateRole(member, 'participant')" 
                      :loading="updatingRoleId === member.user_id"
                      label="Make Participant" 
                      icon="pi pi-user" 
                      severity="secondary" 
                      size="small" 
                      outlined 
                    />

                    <!-- Remove Member -->
                    <Button 
                      @click="openRemoveMemberModal(member)" 
                      label="Remove" 
                      icon="pi pi-user-minus" 
                      severity="danger" 
                      size="small" 
                      text 
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 2: Invitations Manifest (Loaded from dedicated trip_invitations table) -->
          <div class="members-sub-section" style="margin-top: 32px;" v-if="invitationsList.length > 0">
            <div class="sub-section-header">
              <h4 class="sub-deck-title brand-font">📬 Invitation Manifest & Status ({{ invitationsList.length }})</h4>
            </div>

            <div class="members-grid-stack">
              <div 
                v-for="inv in invitationsList" 
                :key="inv.id" 
                class="member-neu-card invitation-item-card"
              >
                <div class="member-info">
                  <div class="member-header">
                    <span class="member-name brand-font">{{ inv.invitee_email.split('@')[0] }}</span>
                    <div class="member-badges-row">
                      <Tag 
                        :value="inv.role === 'munimji' ? 'Munimji' : inv.role === 'co_reviewer' ? 'Co-Reviewer' : 'Participant'" 
                        :severity="inv.role === 'munimji' ? 'warn' : inv.role === 'co_reviewer' ? 'info' : 'secondary'"
                        rounded
                      />
                      <Tag 
                        :value="inv.status === 'pending' ? 'Pending' : inv.status === 'accepted' ? 'Accepted' : inv.status === 'rejected' ? 'Declined' : 'Expired'" 
                        :severity="inv.status === 'pending' ? 'warn' : inv.status === 'accepted' ? 'success' : 'danger'"
                        :icon="inv.status === 'pending' ? 'pi pi-clock' : inv.status === 'accepted' ? 'pi pi-check' : 'pi pi-times'"
                        rounded
                      />
                    </div>
                  </div>
                  <span class="member-email">{{ inv.invitee_email }}</span>
                </div>

                <div class="member-card-footer">
                  <span class="member-join-date mono-num">
                    Sent {{ new Date(inv.created_at).toLocaleDateString() }}
                  </span>

                  <!-- STRICT REQUIREMENT: Accept / Reject buttons visible ONLY to the invited person themselves, NEVER to Munimji or other members -->
                  <div 
                    v-if="authStore.user?.email.toLowerCase() === inv.invitee_email.toLowerCase() && inv.status === 'pending'" 
                    class="member-action-buttons"
                  >
                    <Button 
                      @click="handleRespondInvitation(selectedTrip.id, 'accept')" 
                      :loading="respondingInvitation" 
                      label="Accept" 
                      icon="pi pi-check" 
                      severity="success" 
                      size="small" 
                    />
                    <Button 
                      @click="handleRespondInvitation(selectedTrip.id, 'reject')" 
                      :loading="respondingInvitation" 
                      label="Decline" 
                      icon="pi pi-times" 
                      severity="danger" 
                      size="small" 
                      outlined
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- TAB 4: End-of-Trip Settlement Protocol ("Who Owes Whom") -->
        <section v-if="activeTab === 'settlement'" class="settlement-protocol-deck">
          <div class="deck-header-row">
            <div>
              <h3 class="deck-title brand-font">Expedition Treasury & Settlement Protocol</h3>
              <p class="deck-caption">Authoritative reconciliation of central treasury funds, approved personal claims, and participant cost sharing.</p>
            </div>
            <Button 
              @click="loadSettlement(selectedTrip.id)" 
              :loading="loadingSettlement" 
              label="Recalculate" 
              icon="pi pi-refresh" 
              size="small" 
              outlined 
            />
          </div>

          <!-- Zero Financial Transactions Empty State -->
          <div 
            v-if="settlementData && settlementData.total_trip_cost_paise === 0 && (!settlementData.treasury_summary || settlementData.treasury_summary.total_contributions_paise === 0) && (!settlementData.approved_claims_paise || settlementData.approved_claims_paise === 0)" 
            class="settlement-empty-card"
          >
            <div class="settlement-empty-icon-wrap">
              <i class="pi pi-calculator"></i>
            </div>
            <h4 class="settlement-empty-title brand-font">No Financial Activity to Settle</h4>
            <p class="settlement-empty-desc">
              Settlement calculations require at least one recorded pool contribution, common expense, or approved personal claim. Once transactions are logged, the automated settlement engine will calculate cost equalizations and reimbursements here.
            </p>
            <div class="settlement-empty-actions">
              <Button 
                @click="openContributionModal()" 
                label="Record Inflow" 
                icon="pi pi-arrow-down-left" 
                size="small" 
                outlined 
              />
              <Button 
                @click="openExpenseModal()" 
                label="Log Expense" 
                icon="pi pi-arrow-up-right" 
                size="small" 
                outlined 
              />
              <Button 
                @click="openClaimModal()" 
                label="File Claim" 
                icon="pi pi-plus" 
                size="small" 
                outlined 
              />
            </div>
          </div>

          <!-- Active Settlement Protocol (When Activity Exists) -->
          <div v-else-if="settlementData">
            <!-- Settlement Summary Cards -->
            <div class="settlement-overview-grid">
            <div class="metric-card">
              <span class="metric-heading">TOTAL EXPEDITION COST</span>
              <div class="metric-number mono-num">{{ formatINR(settlementData.total_trip_cost_paise) }}</div>
              <span class="metric-sub">Pool spend ({{ formatINR(settlementData.group_spend_paise || 0) }}) + Claims ({{ formatINR(settlementData.approved_claims_paise || 0) }})</span>
            </div>
            <div class="metric-card">
              <span class="metric-heading">FAIR SHARE PER PERSON</span>
              <div class="metric-number mono-num">{{ formatINR(Math.round(settlementData.equal_share_per_user * 100)) }}</div>
              <span class="metric-sub">Split equally across {{ settlementData.member_count }} members</span>
            </div>
            <div class="metric-card">
              <span class="metric-heading">CENTRAL TREASURY CASH</span>
              <div class="metric-number mono-num">{{ formatINR(settlementData.treasury_summary?.available_cash_paise || 0) }}</div>
              <span class="metric-sub">Held in pool for claims & refunds</span>
            </div>
            <div class="metric-card">
              <span class="metric-heading">APPROVED CLAIMS OWED</span>
              <div class="metric-number mono-num">{{ formatINR(settlementData.approved_claims_paise || 0) }}</div>
              <span class="metric-sub">Personal expenses owed by Group</span>
            </div>
          </div>

          <!-- SECTION 1: Approved Personal Out-of-Pocket Claims (Owed by Group) -->
          <div class="settlement-section-block">
            <div class="section-title-wrap">
              <h4 class="sub-deck-title brand-font">🏆 Approved Personal Expense Claims (Owed by Group)</h4>
              <span class="section-subtitle-hint">
                <i class="pi pi-info-circle"></i> Personal expenses paid out of pocket by participants. Once accepted, these are group liabilities owed by the Expedition Group Treasury to the claimant (NOT owed by the individual reviewer who accepted it).
              </span>
            </div>

            <div v-if="!settlementData || !settlementData.group_claims || settlementData.group_claims.length === 0" class="empty-records-msg">
              No personal out-of-pocket claims recorded for this expedition.
            </div>
            <div v-else class="group-claims-grid">
              <div v-for="c in settlementData.group_claims" :key="c.claim_id" class="group-claim-card">
                <div class="group-claim-header">
                  <div class="claim-title-cluster">
                    <span class="claim-item-title">{{ c.title }}</span>
                    <span class="claim-item-cat">{{ c.category }}</span>
                  </div>
                  <div class="claim-item-amount mono-num">{{ formatINR(c.amount_paise) }}</div>
                </div>
                <div class="group-claim-body">
                  <div class="claim-claimant-info">
                    <span class="info-label">Claimant (Paid Out of Pocket):</span>
                    <span class="info-val font-bold">{{ c.claimant_name }}</span>
                  </div>
                  <div class="claim-debtor-info">
                    <span class="info-label">Owed By:</span>
                    <Tag severity="info" :value="c.owed_by" icon="pi pi-users" class="group-owed-tag" />
                  </div>
                </div>
                <div class="group-claim-footer">
                  <Tag 
                    :severity="c.payable_from_pool ? 'success' : 'warn'" 
                    :icon="c.payable_from_pool ? 'pi pi-check-circle' : 'pi pi-exclamation-triangle'" 
                    :value="c.payable_from_pool ? 'Payable from Central Treasury Cash' : 'Split Across Group Members'" 
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- SECTION 2: Recommended Settlement Actions & Transfers -->
          <div class="settlement-section-block">
            <div class="section-title-wrap">
              <h4 class="sub-deck-title brand-font">💸 Recommended Settlement Actions & Direct Transfers</h4>
              <span class="section-subtitle-hint">
                Reconciliation of Group Treasury claim payouts, unspent advance contribution refunds, and member fair share cost equalization.
              </span>
            </div>

            <!-- Clarification Notice Banner -->
            <div class="settlement-clarification-banner">
              <div class="clarification-item">
                <i class="pi pi-receipt info-icon"></i>
                <div class="clarification-text">
                  <span class="clarification-title">Group Claims:</span>
                  <span>Paid from pool cash to reimburse personal out-of-pocket expenses.</span>
                </div>
              </div>
              <div class="clarification-item">
                <i class="pi pi-replay success-icon"></i>
                <div class="clarification-text">
                  <span class="clarification-title">Surplus Refund:</span>
                  <span>Returns unspent pool cash to the original <em>contributor(s)</em> based on advance deposit share (not role).</span>
                </div>
              </div>
              <div class="clarification-item">
                <i class="pi pi-arrows-h warn-icon"></i>
                <div class="clarification-text">
                  <span class="clarification-title">Fair Share Dues:</span>
                  <span>Direct peer transfers so each participant pays their exact equal share.</span>
                </div>
              </div>
            </div>

            <div v-if="!settlementData || settlementData.transfers.length === 0" class="empty-records-msg">
              🎉 Everyone is fully settled up! No transfers or payments are currently pending.
            </div>
            <div v-else class="transfers-card-grid">
              <div v-for="(t, idx) in settlementData.transfers" :key="idx" class="transfer-neu-tile">
                <!-- Header: Type Tag & Prominent Amount -->
                <div class="transfer-tile-header">
                  <div class="transfer-type-banner">
                    <Tag 
                      v-if="t.transfer_type === 'group_claim'" 
                      severity="info" 
                      icon="pi pi-receipt" 
                      value="Group Claim Reimbursement" 
                    />
                    <Tag 
                      v-else-if="t.transfer_type === 'pool_refund'" 
                      severity="success" 
                      icon="pi pi-replay" 
                      value="Treasury Surplus Refund" 
                    />
                    <Tag 
                      v-else 
                      severity="warn" 
                      icon="pi pi-arrows-h" 
                      value="Fair Share Cost Equalization" 
                    />
                  </div>
                  <div class="transfer-amount-badge mono-num">
                    {{ formatINR(t.amount_paise) }}
                  </div>
                </div>

                <!-- Structured Flow: From -> To -->
                <div class="transfer-flow-container">
                  <div class="party-box source-box">
                    <span class="party-badge debtor">OWED BY / FROM</span>
                    <div class="party-title-line">
                      <i :class="t.from_user_id === 0 ? 'pi pi-building' : 'pi pi-user'" class="party-icon"></i>
                      <span class="party-name brand-font">{{ t.from_user_name }}</span>
                    </div>
                  </div>

                  <div class="transfer-arrow-divider">
                    <i class="pi pi-arrow-right flow-arrow-icon"></i>
                  </div>

                  <div class="party-box target-box">
                    <span class="party-badge creditor">RECIPIENT</span>
                    <div class="party-title-line">
                      <i :class="t.to_user_id === 0 ? 'pi pi-building' : 'pi pi-user'" class="party-icon"></i>
                      <span class="party-name brand-font">{{ t.to_user_name }}</span>
                    </div>
                  </div>
                </div>

                <!-- Footer with Description / Explanation -->
                <div class="transfer-desc-footer" v-if="t.description">
                  <i class="pi pi-info-circle"></i>
                  <span>{{ t.description }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- SECTION 3: Member Balance Ledger Breakdown -->
          <div class="settlement-section-block">
            <h4 class="sub-deck-title brand-font">📊 Participant Net Balance Manifest</h4>
            <div v-if="settlementData" class="record-table-wrapper">
              <table class="neu-table">
                <thead>
                  <tr>
                    <th>PARTICIPANT</th>
                    <th class="text-right">CONTRIBUTIONS</th>
                    <th class="text-right">PERSONAL CLAIMS</th>
                    <th class="text-right">TOTAL PAID</th>
                    <th class="text-right">FAIR SHARE</th>
                    <th class="text-right">NET POSITION</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="b in settlementData.balances" :key="b.user_id">
                    <td class="font-bold brand-font">
                      {{ b.user_name }}
                      <span class="sub-email">({{ b.email }})</span>
                    </td>
                    <td class="mono-num text-right">{{ formatINR(Math.round(b.contributed * 100)) }}</td>
                    <td class="mono-num text-right">{{ formatINR(Math.round(b.claimed * 100)) }}</td>
                    <td class="mono-num text-right font-bold">{{ formatINR(Math.round(b.total_paid * 100)) }}</td>
                    <td class="mono-num text-right text-muted">{{ formatINR(Math.round(b.equal_share * 100)) }}</td>
                    <td :class="['mono-num text-right font-bold', b.net_balance >= 0 ? 'amount-inflow' : 'amount-outflow']">
                      <div class="net-position-cluster">
                        <span>{{ b.net_balance >= 0 ? '+ To Receive ' : '- Owes ' }}{{ formatINR(Math.abs(b.net_balance_paise)) }}</span>
                        <div class="net-pills-row" v-if="b.net_balance >= 0">
                          <span v-if="b.claim_payout && b.claim_payout > 0" class="sub-breakdown-pill claim-pill">
                            Claims: {{ formatINR(Math.round(b.claim_payout * 100)) }}
                          </span>
                          <span v-if="b.pool_refund && b.pool_refund > 0" class="sub-breakdown-pill refund-pill">
                            Refund: {{ formatINR(Math.round(b.pool_refund * 100)) }}
                          </span>
                          <span v-if="(b.net_balance - (b.claim_payout || 0) - (b.pool_refund || 0)) > 0.01" class="sub-breakdown-pill dues-pill">
                            Peer Dues: {{ formatINR(Math.round((b.net_balance - (b.claim_payout || 0) - (b.pool_refund || 0)) * 100)) }}
                          </span>
                        </div>
                        <div class="net-pills-row" v-else>
                          <span class="sub-breakdown-pill owes-pill">
                            Unpaid Fair Share of Trip Cost
                          </span>
                        </div>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          </div>
        </section>
        </template>
      </main>

      <!-- Empty State if no trips selected -->
      <main class="ledger-column empty-workspace" v-else>
        <!-- Pending Invitations Deck when user has pending invites -->
        <div v-if="myPendingInvitations.length > 0" class="pending-invites-hero-deck">
          <div class="pending-invites-hero-header">
            <div class="hero-icon-badge">
              <i class="pi pi-envelope"></i>
            </div>
            <div>
              <h2 class="hero-title brand-font">Expedition Invitations ({{ myPendingInvitations.length }})</h2>
              <p class="hero-sub">You have pending invitations to join shared expeditions. Accept to enter the group workspace and track expenses.</p>
            </div>
          </div>

          <div class="pending-invitations-cards-grid">
            <div v-for="inv in myPendingInvitations" :key="'hero-inv-' + inv.id" class="pending-invitation-hero-card">
              <div class="hero-card-top">
                <div class="hero-card-title-group">
                  <span class="hero-trip-title brand-font">{{ inv.trip_title || 'Expedition #' + inv.trip_id }}</span>
                  <div class="hero-route" v-if="inv.start_location && inv.destination">
                    <i class="pi pi-map-marker"></i> {{ inv.start_location }} ➔ {{ inv.destination }}
                  </div>
                </div>
                <Tag 
                  :value="inv.role === 'co_reviewer' ? 'Co-Reviewer (Auditor)' : 'Participant'" 
                  :severity="inv.role === 'co_reviewer' ? 'info' : 'secondary'"
                  rounded
                />
              </div>

              <div class="hero-inv-details">
                <div class="detail-row">
                  <span class="detail-label">Invited by:</span>
                  <span class="detail-val font-semibold">{{ inv.inviter_name || 'Munimji' }}</span>
                </div>
                <div class="detail-row" v-if="inv.created_at">
                  <span class="detail-label">Received:</span>
                  <span class="detail-val">{{ new Date(inv.created_at).toLocaleDateString() }}</span>
                </div>
              </div>

              <div class="hero-card-actions">
                <Button 
                  @click="handleRespondInvitation(inv.trip_id, 'accept')"
                  :loading="respondingInvitation"
                  label="Accept & Join Expedition"
                  icon="pi pi-check"
                  severity="success"
                />
                <Button 
                  @click="handleRespondInvitation(inv.trip_id, 'reject')"
                  :loading="respondingInvitation"
                  label="Decline"
                  icon="pi pi-times"
                  severity="danger"
                  outlined
                />
              </div>
            </div>
          </div>

          <Divider style="margin: 32px 0 24px 0;" />

          <div class="plan-alternative-box">
            <p class="text-muted" style="margin-bottom: 12px; font-size: 13px;">Looking to start your own expedition instead?</p>
            <Button 
              @click="openCreateTripWizard" 
              label="Plan New Expedition" 
              icon="pi pi-plus" 
              severity="secondary" 
              outlined
            />
          </div>
        </div>

        <!-- Normal empty state if 0 pending invitations -->
        <div v-else class="empty-workspace-card">
          <div class="empty-icon-box">
            <i class="pi pi-compass"></i>
          </div>
          <h2 class="empty-title brand-font">No Expedition Selected</h2>
          <p class="empty-desc">
            Select an ongoing trip from the sidebar to inspect ledger manifests, audit member receipts, and run peer debt settlement, or plan your next expedition.
          </p>
          <Button 
            @click="openCreateTripWizard" 
            label="Plan New Expedition" 
            icon="pi pi-plus" 
            severity="primary" 
          />
        </div>
      </main>

    </div>



    <!-- 2. Submit Expense Claim Dialog -->
    <Dialog 
      v-model:visible="showClaimModal" 
      modal 
      header="Submit Expense Claim" 
      :style="{ width: '92vw', maxWidth: '36rem' }"
      :closable="true"
    >
      <form @submit.prevent="handleCreateClaim" class="prime-modal-form">
        <div class="form-row">
          <div class="form-control">
            <label class="form-label">Amount (in ₹)</label>
            <InputNumber 
              v-model="newClaim.amount_rupees" 
              :min="1" 
              mode="currency" 
              currency="INR" 
              locale="en-IN" 
              placeholder="Enter claim amount (₹)" 
              required 
              fluid 
            />
          </div>
          <div class="form-control">
            <label class="form-label">Category</label>
            <Select 
              v-model="newClaim.category" 
              :options="claimCategoryOptions" 
              optionLabel="label" 
              optionValue="value" 
              placeholder="Select category..." 
              class="neu-select-prime" 
              fluid 
            >
              <template #option="slotProps">
                <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                  <i :class="slotProps.option.icon" class="text-xs"></i>
                  <span>{{ slotProps.option.label }}</span>
                </div>
              </template>
            </Select>
          </div>
        </div>

        <div class="form-control">
          <label class="form-label">Reason / Description</label>
          <InputText v-model="newClaim.reason" placeholder="e.g. Paid diesel bill at Highway HP Pump" required fluid />
        </div>

        <div class="form-row">
          <div class="form-control">
            <label class="form-label">Expense Date & Time</label>
            <InputText v-model="newClaim.expense_date" type="datetime-local" required fluid />
          </div>
          <div class="form-control">
            <label class="form-label">Paid Via</label>
            <Select 
              v-model="newClaim.payment_method" 
              :options="claimPaymentMethodOptions" 
              optionLabel="label" 
              optionValue="value" 
              placeholder="Select payment method..." 
              class="neu-select-prime" 
              fluid 
            >
              <template #option="slotProps">
                <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                  <i :class="slotProps.option.icon" class="text-xs"></i>
                  <span>{{ slotProps.option.label }}</span>
                </div>
              </template>
            </Select>
          </div>
        </div>

        <div class="form-control">
          <label class="form-label">Receipt / Bill Attachment (Optional)</label>
          <input 
            type="file" 
            accept="image/*,.pdf" 
            @change="handleReceiptFileChange" 
            class="neu-field file-upload-input" 
          />
          <span class="file-hint" v-if="claimReceiptFile">Selected: {{ claimReceiptFile.name }}</span>
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showClaimModal = false" />
          <Button 
            type="submit" 
            :loading="uploadingReceipt" 
            label="Submit to Munimji" 
            icon="pi pi-send" 
            severity="primary" 
          />
        </div>
      </form>
    </Dialog>

    <!-- 3. Reject Expense Claim Dialog -->
    <Dialog 
      v-model:visible="showRejectModal" 
      modal 
      header="Reject Expense Claim" 
      :style="{ width: '92vw', maxWidth: '30rem' }"
      :closable="true"
    >
      <form @submit.prevent="confirmRejectClaim" class="prime-modal-form">
        <Message severity="warn" :closable="false" size="small">
          Provide a clear explanation for the claimant so they understand why this out-of-pocket claim cannot be reimbursed from the pool.
        </Message>

        <div class="form-control">
          <label class="form-label">Reason for Rejection</label>
          <Textarea 
            v-model="rejectionReason" 
            placeholder="e.g. Receipt amount mismatch or personal purchase not included in trip itinerary." 
            rows="3" 
            required 
            fluid 
          />
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showRejectModal = false" />
          <Button type="submit" label="Confirm Rejection" icon="pi pi-times" severity="danger" />
        </div>
      </form>
    </Dialog>

    <!-- 4. Invite Participant Dialog -->
    <Dialog 
      v-model:visible="showInviteModal" 
      modal 
      header="Invite Participant" 
      :style="{ width: '92vw', maxWidth: '34rem' }"
      :closable="true"
    >
      <form @submit.prevent="handleInviteMember" class="prime-modal-form">
        <div class="form-control">
          <label class="form-label">Select Registered User</label>
          <div class="eligible-user-picker">
            <AutoComplete
              v-model="selectedEligibleUser"
              :suggestions="eligibleUsers"
              @complete="handleSearchEligibleUsers"
              @item-select="handleEligibleUserSelect"
              @change="handleEligibleUserChange"
              optionLabel="email"
              dropdown
              placeholder="Click dropdown or search name / email..."
              fluid
              class="eligible-user-autocomplete"
            >
              <template #option="slotProps">
                <div class="eligible-user-option">
                  <div class="eligible-user-avatar">
                    {{ (slotProps.option.name || slotProps.option.email || 'U').slice(0, 2).toUpperCase() }}
                  </div>
                  <div class="eligible-user-meta">
                    <span class="eligible-user-name">{{ slotProps.option.name }}</span>
                    <span class="eligible-user-email">{{ slotProps.option.email }}</span>
                  </div>
                  <Tag severity="info" value="Registered" class="eligible-tag" />
                </div>
              </template>
              <template #empty>
                <div class="eligible-empty-state">
                  <i class="pi pi-users"></i>
                  <span>No non-member registered users match</span>
                </div>
              </template>
            </AutoComplete>
          </div>
          <span class="input-hint-subtle">
            <i class="pi pi-info-circle"></i> Showing registered accounts not yet in this trip. Type to filter via live search or pick from dropdown.
          </span>
        </div>

        <!-- Selected User Confirmation Card -->
        <div v-if="selectedUserMeta" class="selected-user-card">
          <div class="selected-user-avatar">
            {{ (selectedUserMeta.name || selectedUserMeta.email).slice(0, 2).toUpperCase() }}
          </div>
          <div class="selected-user-info">
            <div class="selected-user-name">{{ selectedUserMeta.name }}</div>
            <div class="selected-user-email">{{ selectedUserMeta.email }}</div>
          </div>
          <Tag severity="success" icon="pi pi-check" value="Ready to Invite" />
        </div>

        <div class="form-control">
          <label class="form-label">Role Privilege</label>
          <Select 
            v-model="newInvite.role" 
            :options="rolePrivilegeOptions" 
            optionLabel="label" 
            optionValue="value" 
            class="neu-select-prime" 
            fluid 
          >
            <template #option="slotProps">
              <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                <i :class="slotProps.option.icon" class="text-xs"></i>
                <span>{{ slotProps.option.label }}</span>
              </div>
            </template>
          </Select>
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showInviteModal = false" />
          <Button type="submit" label="Send Invitation" icon="pi pi-send" severity="primary" :disabled="!isInviteReady" />
        </div>
      </form>
    </Dialog>

    <!-- 5. Log Contribution Receipt Dialog -->
    <Dialog 
      v-model:visible="showContributionModal" 
      modal 
      header="Log Contribution Receipt" 
      :style="{ width: '92vw', maxWidth: '34rem' }"
      :closable="true"
    >
      <form @submit.prevent="handleRecordContribution" class="prime-modal-form">
        <div class="form-row">
          <div class="form-control">
            <label class="form-label">Member</label>
            <Select 
              v-model="newContribution.user_id" 
              :options="membersList" 
              :optionLabel="(m: any) => m.user_name || m.email" 
              optionValue="user_id" 
              placeholder="Select participant" 
              class="neu-select-prime" 
              fluid 
            >
              <template #option="slotProps">
                <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                  <i class="pi pi-user text-xs"></i>
                  <span>{{ slotProps.option.user_name || slotProps.option.email }}</span>
                </div>
              </template>
            </Select>
          </div>
          <div class="form-control">
            <label class="form-label">Amount (in ₹)</label>
            <InputNumber 
              v-model="newContribution.amount_rupees" 
              :min="1" 
              mode="currency" 
              currency="INR" 
              locale="en-IN" 
              placeholder="Enter contribution amount (₹)" 
              required 
              fluid 
            />
          </div>
        </div>

        <div class="form-row">
          <div class="form-control">
            <label class="form-label">Payment Method</label>
            <Select 
              v-model="newContribution.payment_method" 
              :options="contributionPaymentOptions" 
              optionLabel="label" 
              optionValue="value" 
              placeholder="Select payment method..." 
              class="neu-select-prime" 
              fluid 
            >
              <template #option="slotProps">
                <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                  <i :class="slotProps.option.icon" class="text-xs"></i>
                  <span>{{ slotProps.option.label }}</span>
                </div>
              </template>
            </Select>
          </div>
          <div class="form-control">
            <label class="form-label">Received At</label>
            <InputText v-model="newContribution.received_at" type="datetime-local" required fluid />
          </div>
        </div>

        <div class="form-control">
          <label class="form-label">Note / Reference (Optional)</label>
          <InputText v-model="newContribution.notes" placeholder="e.g. Google Pay UTR #819283" fluid />
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showContributionModal = false" />
          <Button type="submit" label="Record Inflow" icon="pi pi-check" severity="primary" />
        </div>
      </form>
    </Dialog>

    <!-- 6. Record Group Expense Dialog -->
    <Dialog 
      v-model:visible="showExpenseModal" 
      modal 
      header="Record Group Expense" 
      :style="{ width: '92vw', maxWidth: '34rem' }"
      :closable="true"
    >
      <form @submit.prevent="handleRecordExpense" class="prime-modal-form">
        <div class="form-row">
          <div class="form-control">
            <label class="form-label">Expense Title</label>
            <InputText v-model="newExpense.title" placeholder="e.g. Villa Booking Advance" required fluid />
          </div>
          <div class="form-control">
            <label class="form-label">Amount (in ₹)</label>
            <InputNumber 
              v-model="newExpense.amount_rupees" 
              :min="1" 
              mode="currency" 
              currency="INR" 
              locale="en-IN" 
              placeholder="Enter expense amount (₹)" 
              required 
              fluid 
            />
          </div>
        </div>

        <div class="form-row">
          <div class="form-control">
            <label class="form-label">Category</label>
            <Select 
              v-model="newExpense.category" 
              :options="expenseCategoryOptions" 
              optionLabel="label" 
              optionValue="value" 
              placeholder="Select category..." 
              class="neu-select-prime" 
              fluid 
            >
              <template #option="slotProps">
                <div class="flex items-center gap-2 text-xs font-semibold py-0.5">
                  <i :class="slotProps.option.icon" class="text-xs"></i>
                  <span>{{ slotProps.option.label }}</span>
                </div>
              </template>
            </Select>
          </div>
          <div class="form-control">
            <label class="form-label">Expense Date</label>
            <InputText v-model="newExpense.expense_date" type="datetime-local" required fluid />
          </div>
        </div>

        <div class="form-control">
          <label class="form-label">Additional Notes</label>
          <InputText v-model="newExpense.notes" placeholder="e.g. Paid via Munimji GPay from group funds" fluid />
        </div>

        <Message 
          v-if="newExpense.amount_rupees && (Number(newExpense.amount_rupees) > totalContributionRupees)" 
          severity="warn" 
          :closable="false" 
          size="small"
        >
          ⚠️ <strong>Treasury Deficit Warning:</strong> This expense of ₹{{ Number(newExpense.amount_rupees).toLocaleString('en-IN') }} exceeds the total expedition contributions (₹{{ totalContributionRupees.toLocaleString('en-IN') }}).
        </Message>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showExpenseModal = false" />
          <Button type="submit" label="Save Outflow" icon="pi pi-check" severity="primary" />
        </div>
      </form>
    </Dialog>

    <!-- 7. Remove Member Dialog -->
    <Dialog 
      v-model:visible="showRemoveMemberModal" 
      modal 
      header="Remove Expedition Participant" 
      :style="{ width: '92vw', maxWidth: '32rem' }"
      :closable="true"
    >
      <form @submit.prevent="confirmRemoveMember" class="prime-modal-form">
        <Message severity="warn" :closable="false" size="small">
          Removing this participant will immediately revoke their access and email them the reason. Past recorded contributions and expenses remain preserved.
        </Message>

        <div class="form-control">
          <label class="form-label">Participant to Remove</label>
          <div class="member-preview-box">
            <span class="font-bold brand-font">{{ removingMember?.user_name || removingMember?.email?.split('@')[0] || 'Member' }}</span>
            <span class="text-muted sub-email">({{ removingMember?.email }})</span>
          </div>
        </div>

        <div class="form-control">
          <label class="form-label">Reason for Removal (Mandatory)</label>
          <Textarea 
            v-model="removalReason" 
            placeholder="e.g. Inactivity, non-payment of fair share, or mutually agreed departure from group." 
            rows="3" 
            required 
            fluid 
            autofocus
          />
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showRemoveMemberModal = false" />
          <Button 
            type="submit" 
            :loading="processingRemoval" 
            label="Confirm Removal" 
            icon="pi pi-user-minus" 
            severity="danger" 
          />
        </div>
      </form>
    </Dialog>

    <!-- 8. Edit Profile Dialog -->
    <Dialog 
      v-model:visible="showProfileModal" 
      modal 
      header="Edit My Profile" 
      :style="{ width: '92vw', maxWidth: '30rem' }"
      :closable="true"
    >
      <form @submit.prevent="handleUpdateProfile" class="prime-modal-form">
        <div class="form-control">
          <label class="form-label">Email Address (Read-only)</label>
          <InputText :value="authStore.user?.email" disabled fluid class="opacity-60 cursor-not-allowed" />
        </div>

        <div class="form-control">
          <label class="form-label">Full Name</label>
          <InputText v-model="profileName" placeholder="Your full name" required fluid autofocus />
        </div>

        <div class="form-control">
          <label class="form-label">Phone Number (Optional)</label>
          <InputText v-model="profilePhone" placeholder="+91 98765 43210" fluid />
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showProfileModal = false" />
          <Button 
            type="submit" 
            :loading="savingProfile" 
            label="Save Changes" 
            icon="pi pi-check" 
            severity="primary" 
          />
        </div>
      </form>
    </Dialog>

    <!-- 9. Dedicated Change Password Dialog -->
    <Dialog 
      v-model:visible="showPasswordModal" 
      modal 
      header="Change Password" 
      :style="{ width: '92vw', maxWidth: '30rem' }"
      :closable="true"
    >
      <form @submit.prevent="handleChangePassword" class="prime-modal-form">
        <div class="form-control">
          <label class="form-label">Current Password</label>
          <Password v-model="currentPassword" placeholder="Enter your current password" :feedback="false" toggleMask required fluid autofocus />
        </div>

        <div class="form-control">
          <label class="form-label">New Password</label>
          <Password v-model="newPassword" placeholder="Enter new password (min 6 chars)" :feedback="false" toggleMask required fluid />
        </div>

        <div class="form-control">
          <label class="form-label">Confirm New Password</label>
          <Password v-model="confirmNewPassword" placeholder="Re-type new password" :feedback="false" toggleMask required fluid />
        </div>

        <div class="dialog-actions-row">
          <Button type="button" label="Cancel" severity="secondary" text @click="showPasswordModal = false" />
          <Button 
            type="submit" 
            :loading="changingPassword" 
            label="Update Password" 
            icon="pi pi-lock" 
            severity="primary" 
          />
        </div>
      </form>
    </Dialog>

    <!-- 10. Delete Expedition Confirmation Dialog -->
    <Dialog 
      v-model:visible="showDeleteTripModal" 
      modal 
      header="Delete Expedition" 
      :style="{ width: '92vw', maxWidth: '30rem' }"
      :closable="true"
    >
      <div class="space-y-4">
        <Message severity="error" :closable="false" size="small">
          ⚠️ <strong>Irreversible Action:</strong> Deleting this expedition will permanently erase the entire manifest, member contributions pool, recorded group expenses, and audit logs.
        </Message>

        <p class="text-sm text-[var(--text-main)]">
          Are you sure you want to permanently delete <strong class="font-bold brand-font text-red-400">{{ selectedTrip?.title }}</strong>?
        </p>

        <div class="dialog-actions-row pt-2">
          <Button type="button" label="Cancel" severity="secondary" text @click="showDeleteTripModal = false" />
          <Button 
            type="button" 
            @click="handleDeleteTrip"
            :loading="deletingTrip" 
            label="Confirm & Delete" 
            icon="pi pi-trash" 
            severity="danger" 
          />
        </div>
      </div>
    </Dialog>

    <!-- 11. Receipt Lightbox Preview Dialog -->
    <Dialog 
      v-model:visible="showReceiptModal" 
      modal 
      :header="activeReceipt?.title || 'Receipt Attachment'" 
      :style="{ width: '92vw', maxWidth: '38rem' }"
      :closable="true"
    >
      <div v-if="activeReceipt" class="receipt-modal-body">
        <div class="receipt-meta-banner">
          <div v-if="activeReceipt.claimant" class="receipt-claimant">
            <span class="text-xs text-muted">Claimant: </span>
            <strong class="font-bold brand-font text-sm">{{ activeReceipt.claimant }}</strong>
          </div>
          <div v-if="activeReceipt.amount" class="receipt-amount mono-num font-bold">
            {{ formatINR(activeReceipt.amount) }}
          </div>
        </div>

        <div class="receipt-content-box">
          <iframe 
            v-if="activeReceipt.isPdf" 
            :src="activeReceipt.url" 
            class="receipt-pdf-frame"
          ></iframe>
          <img 
            v-else 
            :src="activeReceipt.url" 
            :alt="activeReceipt.title" 
            class="receipt-image-preview" 
          />
        </div>

        <div class="dialog-actions-row pt-3">
          <Button 
            type="button" 
            label="Close" 
            severity="secondary" 
            text 
            @click="showReceiptModal = false" 
          />
          <a 
            :href="activeReceipt.url" 
            target="_blank" 
            rel="noopener noreferrer" 
            class="receipt-external-btn"
          >
            <i class="pi pi-external-link mr-1"></i> Open Original File
          </a>
        </div>
      </div>
    </Dialog>
  </div>
</template>

<style scoped>
.app-container {
  min-height: 100vh;
  background-color: var(--bg);
  color: var(--text-main);
  display: flex;
  flex-direction: column;
}

/* Header */
.app-header {
  height: 64px;
  background-color: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 28px;
  position: sticky;
  top: 0;
  z-index: 50;
}

.header-start {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mobile-menu-btn {
  display: none;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 16px;
}

.brand-cluster {
  display: flex;
  align-items: center;
  gap: 10px;
}

.brand-icon-badge {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 3px;
}

.brand-logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 6px;
}

.brand-title {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-main);
  letter-spacing: -0.02em;
}

.brand-divider {
  color: var(--text-muted);
  opacity: 0.4;
  margin: 0 2px;
}

.header-breadcrumb-tag {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-caption {
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 500;
}

.header-trip-pill {
  font-size: 12px;
  font-weight: 600;
  padding: 3px 10px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  border-radius: 6px;
  color: var(--text-main);
  max-width: 220px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* User Profile Header Button */
.user-profile-btn {
  display: flex;
  align-items: center;
  gap: 9px;
  background: var(--surface);
  border: 1px solid transparent;
  box-shadow: var(--shadow-flat-sm);
  padding: 4px 14px 4px 5px;
  border-radius: 999px;
  cursor: pointer;
  color: var(--text-main);
  font-family: inherit;
  transition: all 0.2s ease;
}

.user-profile-btn:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.3);
}

.user-profile-btn:active {
  transform: translateY(1px);
  box-shadow: var(--shadow-inset);
}

.user-avatar-chip {
  background: var(--accent) !important;
  color: #ffffff !important;
  font-weight: 700;
  font-size: 11px;
  width: 28px !important;
  height: 28px !important;
}

.user-email-text {
  font-size: 13px;
  font-weight: 600;
  max-width: 170px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-main);
}

.user-caret {
  font-size: 10px;
  color: var(--text-muted);
  transition: transform 0.2s ease;
}

/* Popover Profile Panel */
.popover-profile-card {
  width: 270px;
  padding: 2px;
}

.popover-header-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.popover-avatar-lg {
  background: var(--accent) !important;
  color: #ffffff !important;
  font-weight: 700;
}

.popover-info-col {
  display: flex;
  flex-direction: column;
  gap: 3px;
  flex: 1;
  overflow: hidden;
}

.popover-display-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.popover-display-email {
  font-size: 12px;
  color: var(--text-muted);
  word-break: break-all;
}

.popover-tag-row {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}

.popover-sep {
  margin: 10px 0 !important;
}

.popover-menu-stack {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.popover-menu-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 12px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--text-main);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s ease;
  text-align: left;
}

.popover-menu-btn:hover {
  background: rgba(0, 0, 0, 0.05);
}

html.dark .popover-menu-btn:hover {
  background: rgba(255, 255, 255, 0.07);
}

.popover-menu-btn.text-danger {
  color: #ef4444;
}

.popover-menu-btn.text-danger:hover {
  background: rgba(239, 68, 68, 0.08);
}

/* Empty Workspace State */
.empty-workspace {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 64px);
  padding: 40px 20px;
}

.empty-workspace-card {
  max-width: 440px;
  text-align: center;
  padding: 48px 36px;
  background: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.empty-icon-box {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  color: var(--accent);
}

.empty-title {
  margin: 0;
  font-size: 22px;
  color: var(--text-main);
}

.empty-desc {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
  line-height: 1.6;
}

/* General Neumorphic Buttons */
.neu-btn {
  background: var(--surface);
  border: none;
  box-shadow: var(--shadow-flat-sm);
  color: var(--text-main);
  cursor: pointer;
  font-family: inherit;
  font-weight: 600;
  transition: all 0.18s ease;
}

.neu-btn:hover {
  transform: translateY(-1px);
}

.neu-btn:active {
  transform: translateY(1px);
  box-shadow: var(--shadow-inset);
}

.header-pill-btn {
  border-radius: 999px;
  padding: 8px 18px;
  font-size: 13px;
}

.action-create {
  color: var(--accent);
}

/* App Layout Structure */
.app-content-layout {
  display: flex;
  flex: 1;
  padding: 24px 28px;
  gap: 24px;
  width: 100%;
  margin: 0;
}

/* Center & Right: Ledger Column Main Content */
.ledger-column {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 22px;
  width: 100%;
}

/* Left Sidebar Column */
.sidebar-column {
  width: 290px;
  flex-shrink: 0;
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 20px;
  height: fit-content;
  max-height: calc(100vh - 120px);
  display: flex;
  flex-direction: column;
}

.sidebar-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding: 0 4px;
}

.sidebar-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.sidebar-badge {
  font-size: 11px;
  background-color: var(--active-pill);
  box-shadow: var(--shadow-inset);
  padding: 2px 8px;
  border-radius: 6px;
  color: var(--text-muted);
}

.trip-items-scroll {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 240px;
  overflow-y: auto;
}

/* Sidebar Section Navigation */
.sidebar-section-nav {
  margin-top: 20px;
  padding-top: 18px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.nav-pills-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sidebar-nav-pill {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--surface);
  border: none;
  box-shadow: var(--shadow-flat-sm);
  border-radius: 12px;
  padding: 12px 14px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main);
  cursor: pointer;
  font-family: inherit;
  transition: all 0.18s ease;
}

.sidebar-nav-pill:hover {
  transform: translateY(-1px);
}

.sidebar-nav-pill.active {
  box-shadow: var(--shadow-inset);
  background: var(--active-pill);
  color: var(--accent);
}

.nav-badge {
  font-size: 11px;
  box-shadow: var(--shadow-inset);
  padding: 1px 7px;
  border-radius: 6px;
  color: var(--text-muted);
}

.sidebar-pill-content {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.sidebar-pill-content i {
  font-size: 14px;
  color: var(--text-muted);
  transition: color 0.18s ease;
}

.sidebar-nav-pill.active .sidebar-pill-content i {
  color: var(--accent);
}

.nav-count-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-sizing: border-box;
  transition: all 0.18s ease;
}

.sidebar-nav-pill.active .nav-count-pill,
.workspace-tab-btn.active .nav-count-pill {
  background: rgba(0, 240, 255, 0.14);
  color: var(--accent);
  border-color: rgba(0, 240, 255, 0.3);
}


.trip-neu-item {
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border-radius: 14px;
  padding: 14px;
  cursor: pointer;
  transition: all 0.18s ease;
}

.trip-neu-item:hover {
  transform: translateY(-1px);
}

.trip-neu-item.active {
  box-shadow: var(--shadow-inset);
  background: var(--active-pill);
}

.trip-item-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.trip-item-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-main);
}

.trip-item-status {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--text-muted);
}

.trip-item-route {
  font-size: 12px;
  color: var(--text-muted);
}

/* ==========================================================================
   Trip Creation Wizard (Option A - 30/70 Split Layout)
   ========================================================================== */
.wizard-split-layout {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 24px;
  width: 100%;
  max-width: 1300px;
  margin: 0 auto;
  align-items: stretch;
  animation: fadeInWizard 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes fadeInWizard {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Left 30%: Visual Card */
.wizard-sidebar-visual {
  background: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 28px 24px;
  display: flex;
  flex-direction: column;
  position: relative;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.04));
  overflow: hidden;
}

.visual-card-wrapper {
  display: flex;
  flex-direction: column;
  height: 100%;
  justify-content: space-between;
}

.step-visual-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Left 30%: Option C Boarding Pass Styles */
.wizard-ticket-preview-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  justify-content: flex-start;
}

.ticket-header-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  padding: 5px 12px;
  border-radius: 20px;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.12em;
  color: var(--text-muted);
  width: fit-content;
}

.live-dot-pulse {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px #10b981;
  animation: pulseDot 2s infinite ease-in-out;
}

@keyframes pulseDot {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.85); }
}

.pass-card {
  background: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 16px;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.06));
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.pass-top-stub {
  padding: 16px 18px 12px 18px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
}

.pass-airline-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pass-logo-badge {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent);
  font-size: 14px;
}

.pass-brand-details {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pass-expedition-label {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.1em;
  color: var(--text-muted);
}

.pass-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-main);
  max-width: 140px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pass-status-tag {
  font-size: 9px !important;
  font-weight: 700 !important;
  letter-spacing: 0.05em !important;
  padding: 2px 8px !important;
}

.pass-route-deck {
  padding: 14px 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.pass-route-city {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 90px;
}

.pass-city-label {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.pass-city-code {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 0.05em;
  font-family: monospace;
  color: var(--text-main);
}

.pass-city-name {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pass-flight-path {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  flex: 1;
  padding: 0 4px;
}

.pass-flight-path i {
  font-size: 12px;
  color: var(--accent);
}

.pass-flight-line {
  width: 100%;
  height: 1px;
  border-top: 1px dashed var(--border-subtle, rgba(255, 255, 255, 0.2));
}

.pass-transport-mode {
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

/* Perforation Divider */
.pass-perforation-divider {
  position: relative;
  height: 20px;
  display: flex;
  align-items: center;
  margin: 4px 0;
}

.perforation-notch {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--surface);
  box-shadow: inset 2px 2px 4px var(--shadow-dark), inset -2px -2px 4px var(--shadow-light);
  position: absolute;
  top: 2px;
}

.left-notch {
  left: -8px;
}

.right-notch {
  right: -8px;
}

.perforation-dashed-line {
  width: 100%;
  margin: 0 16px;
  border-top: 2px dashed rgba(255, 255, 255, 0.12);
}

/* Meta Grid */
.pass-meta-grid {
  padding: 12px 18px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 10px;
  background: rgba(0, 0, 0, 0.03);
}

.pass-meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-field-label {
  font-size: 8.5px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.meta-field-val {
  font-size: 12px;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Barcode */
.pass-barcode-footer {
  padding: 14px 18px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.04));
}

.barcode-graphic {
  display: flex;
  align-items: center;
  gap: 3px;
  height: 32px;
  opacity: 0.75;
}

.bar {
  background: var(--text-main);
  height: 100%;
  border-radius: 1px;
}

.b1 { width: 1.5px; }
.b2 { width: 3px; }
.b3 { width: 4.5px; }
.b4 { width: 6px; }

.pass-serial-num {
  font-size: 9px;
  letter-spacing: 0.15em;
  font-family: monospace;
  color: var(--text-muted);
}

.ticket-bottom-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.4;
  padding: 6px 4px;
}

/* Right 70%: Wizard Surface Card */
.wizard-surface-card {
  background: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 32px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  position: relative;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.04));
  width: 100%;
}

.wizard-top-nav {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.05));
}

.wizard-branding {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.wizard-badge {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.12em;
  color: var(--accent);
  text-transform: uppercase;
}

.wizard-headline {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-main);
  margin: 0;
}

.wizard-cancel-btn {
  font-size: 12px !important;
  font-weight: 600 !important;
  border-radius: 8px !important;
}

/* Stepper Bar */
.wizard-stepper {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  border-radius: 14px;
  gap: 12px;
}

.stepper-node {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.stepper-node:hover:not(.node-disabled) {
  background: rgba(255, 255, 255, 0.03);
}

.stepper-node.node-disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.node-circle {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  color: var(--text-muted);
  transition: all 0.2s ease;
}

.node-active .node-circle {
  box-shadow: var(--shadow-inset);
  background: var(--accent);
  color: #fff;
}

.node-completed .node-circle {
  background: #10b981;
  color: #ffffff;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.35);
}

.node-meta {
  display: flex;
  flex-direction: column;
}

.node-step-label {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.node-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main);
  white-space: nowrap;
}

.node-active .node-title {
  color: var(--accent);
  font-weight: 700;
}

.stepper-connector {
  flex: 1;
  height: 2px;
  background: var(--border-subtle, rgba(255, 255, 255, 0.08));
  transition: all 0.3s ease;
}

.connector-completed {
  background: #10b981;
}

/* Step Body */
.wizard-step-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding-top: 4px;
}

.step-intro {
  display: flex;
  align-items: center;
  gap: 14px;
}

.step-icon-badge {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: var(--accent);
  flex-shrink: 0;
}

.step-heading {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-main);
  margin: 0 0 2px 0;
}

.step-subheading {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
  margin: 0;
  max-width: 680px;
}

.wizard-fields-cluster {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.wizard-two-col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.wizard-neu-input {
  box-shadow: var(--shadow-inset) !important;
  background: var(--surface) !important;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.06)) !important;
  border-radius: 10px !important;
  padding: 10px 14px !important;
  font-size: 13px !important;
  color: var(--text-main) !important;
  transition: all 0.18s ease;
}

.wizard-neu-input:focus {
  border-color: var(--accent) !important;
  box-shadow: var(--shadow-inset), 0 0 0 1px var(--accent) !important;
}

.wizard-title-input {
  font-size: 15px !important;
  font-weight: 600 !important;
  padding: 12px 16px !important;
}

.wizard-route-preview {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border-radius: 10px;
  padding: 10px 16px;
  margin-top: 4px;
}

.preview-route-badge {
  display: flex;
  align-items: center;
  gap: 8px;
}

.preview-node {
  box-shadow: var(--shadow-inset);
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  color: var(--text-main);
}

.preview-tagline {
  font-size: 11px;
  font-weight: 600;
  color: #10b981;
}

/* Summary Card in Step 3 */
.wizard-summary-card {
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  border-radius: 14px;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.04));
}

.summary-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.summary-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
}

.summary-details-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
}

.summary-col {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.summary-dim {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.summary-val {
  font-size: 12px;
  color: var(--text-main);
}

/* Actions Bar */
.wizard-actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 14px;
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.05));
  margin-top: 4px;
}

.btn-wizard-back-small {
  font-size: 12px !important;
  font-weight: 600 !important;
  padding: 8px 16px !important;
  border-radius: 8px !important;
}

.btn-wizard-next, .btn-wizard-launch {
  font-weight: 700 !important;
  padding: 10px 22px !important;
  border-radius: 10px !important;
  font-size: 13px !important;
}

/* STEP 4: Success Deck */
.wizard-success-deck {
  text-align: center;
  align-items: center;
  padding: 16px 8px 8px 8px;
  gap: 16px;
}

.success-icon-badge {
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: #10b981;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  box-shadow: 0 0 20px rgba(16, 185, 129, 0.35);
  margin-bottom: 2px;
}

.success-kicker {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.12em;
  color: #10b981;
  text-transform: uppercase;
}

.success-headline {
  font-size: 24px;
  font-weight: 800;
  color: var(--text-main);
  margin: 0;
}

.success-trip-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--accent);
  margin: 2px 0 10px 0;
}

.success-description {
  max-width: 540px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-muted);
  margin: 0 auto 16px auto;
}

.success-manifest-strip {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
  margin-bottom: 20px;
}

.manifest-pill {
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  padding: 6px 14px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main);
  display: flex;
  align-items: center;
  gap: 6px;
}

.success-actions-row {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.btn-enter-workspace {
  font-weight: 700 !important;
  padding: 14px 28px !important;
  border-radius: 12px !important;
  box-shadow: 0 4px 14px rgba(99, 102, 241, 0.3) !important;
}

@media (max-width: 900px) {
  .wizard-split-layout {
    grid-template-columns: 1fr;
    max-width: 100%;
  }

  .wizard-sidebar-visual {
    display: none;
  }
}

@media (max-width: 768px) {
  .wizard-surface-card {
    padding: 22px 18px;
  }

  .wizard-stepper {
    flex-direction: row !important;
    align-items: center !important;
    justify-content: space-between !important;
    padding: 6px 10px !important;
    gap: 4px !important;
    border-radius: 12px !important;
  }

  .stepper-node {
    padding: 4px 6px !important;
    gap: 6px !important;
    flex: 1 !important;
    justify-content: center !important;
  }

  .node-circle {
    width: 24px !important;
    height: 24px !important;
    min-width: 24px !important;
    font-size: 11px !important;
  }

  .node-step-label {
    display: none !important;
  }

  .node-title {
    font-size: 11px !important;
    white-space: nowrap !important;
  }

  .stepper-connector {
    display: block !important;
    flex: 1 !important;
    height: 1px !important;
    min-width: 6px !important;
    background: rgba(255, 255, 255, 0.08) !important;
  }

  .wizard-two-col {
    grid-template-columns: 1fr;
  }

  .summary-details-grid {
    grid-template-columns: 1fr 1fr;
  }

  .wizard-actions-bar {
    flex-direction: column-reverse;
    gap: 12px;
  }

  .wizard-actions-bar button {
    width: 100%;
  }

  .success-actions-row {
    flex-direction: column;
    width: 100%;
  }

  .success-actions-row button {
    width: 100%;
  }
}

/* Hero Section */
.trip-hero-section {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 30px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.hero-badge {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.1em;
  color: var(--text-muted);
}

.hero-title {
  margin: 4px 0 10px 0;
  font-size: 26px;
  font-weight: 700;
}

.hero-route-line {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--text-muted);
}

.route-node {
  box-shadow: var(--shadow-inset);
  padding: 3px 10px;
  border-radius: 6px;
  font-weight: 600;
  color: var(--text-main);
}

.route-sep {
  opacity: 0.4;
  font-size: 11px;
}

.treasurer-chip {
  box-shadow: var(--shadow-flat-sm);
  padding: 10px 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.chip-role {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.chip-name {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
}

/* Financial Metrics Grid (Responsive 4 -> 2 -> 1) */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 18px;
}

.metric-card {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 18px;
  padding: 20px;
  display: flex;
  flex-direction: column;
}

.card-focal {
  box-shadow: var(--shadow-flat-lg);
}

.metric-heading {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  margin-bottom: 10px;
}

.focal-heading {
  color: var(--text-muted);
}

.metric-number {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-main);
  margin-bottom: 6px;
}

.focal-number {
  color: var(--text-main);
  font-weight: 700;
}

.metric-sub {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.4;
}

/* Operations Deck */
.operations-deck {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 30px;
}

.deck-header {
  margin-bottom: 20px;
}

.deck-title {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 700;
}

.deck-caption {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.action-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 18px;
}

.action-tile {
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border-radius: 16px;
  padding: 20px;
  display: flex;
  flex-direction: column;
}

.tile-category {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.tile-title {
  margin: 4px 0 10px 0;
  font-size: 15px;
  font-weight: 700;
}

.tile-text {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
  margin: 0 0 18px 0;
  flex: 1;
}

.tile-btn {
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  width: 100%;
}

.focal-btn {
  color: var(--accent);
}

/* Claims Review Deck Styling */
.claims-review-deck {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 30px;
}

.empty-claims-msg {
  font-size: 13px;
  color: var(--text-muted);
  padding: 16px 0;
}

.claims-list-stack {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.claim-neu-card {
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border-radius: 16px;
  padding: 18px 22px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.claim-card-left {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.claim-user-tag {
  display: flex;
  align-items: center;
  gap: 10px;
}

.claimant-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-main);
}

.claim-status-pill {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  padding: 2px 8px;
  border-radius: 6px;
  box-shadow: var(--shadow-inset);
}

.claim-status-pill.approved {
  color: #10b981;
}

.claim-status-pill.pending {
  color: #f59e0b;
}

.claim-status-pill.rejected {
  color: #ef4444;
}

.claim-reason {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-main);
}

.claim-meta-details {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--text-muted);
}

.category-badge {
  box-shadow: var(--shadow-flat-sm);
  padding: 2px 8px;
  border-radius: 6px;
  font-weight: 600;
}

.receipt-link {
  color: var(--accent);
}

.claim-card-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.claim-amount {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-main);
}

.claim-action-buttons {
  display: flex;
  gap: 8px;
}

.btn-approve {
  border-radius: 8px;
  padding: 6px 14px;
  font-size: 12px;
  color: #10b981;
}

.btn-reject {
  border-radius: 8px;
  padding: 6px 14px;
  font-size: 12px;
  color: #ef4444;
}

/* Invitation Banner */
.invitation-banner-card {
  background: var(--surface);
  box-shadow: var(--shadow-raised);
  border-radius: 20px;
  padding: 24px;
  margin-bottom: 24px;
  border-left: 5px solid #f59e0b;
}

.invitation-banner-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
  flex-wrap: wrap;
}

.invitation-banner-text {
  flex: 1;
  min-width: 260px;
}

.invitation-badge {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: #d97706;
  background: rgba(245, 158, 11, 0.12);
  padding: 4px 10px;
  border-radius: 6px;
  display: inline-block;
  margin-bottom: 8px;
}

.invitation-heading {
  font-size: 20px;
  font-weight: 800;
  color: var(--text-main);
  margin: 0 0 6px 0;
}

.invitation-subtext {
  font-size: 13px;
  color: var(--text-muted);
  margin: 0;
  line-height: 1.5;
}

.invitation-banner-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.btn-accept {
  background: #10b981;
  color: #ffffff;
  font-weight: 700;
  padding: 10px 20px;
  border-radius: 12px;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-accept:hover {
  background: #059669;
  transform: translateY(-1px);
}

.btn-decline {
  background: transparent;
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
  font-weight: 700;
  padding: 10px 18px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-decline:hover {
  background: rgba(239, 68, 68, 0.08);
}

/* Members Governance Deck Styling */
.members-governance-deck {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 30px;
}

.deck-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.btn-invite-member {
  border-radius: 12px;
  padding: 8px 16px;
  font-size: 13px;
  color: var(--accent);
}

.members-grid-stack {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(310px, 1fr));
  gap: 18px;
  width: 100%;
}

.sub-section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.sub-section-tag {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--accent);
  background: rgba(37, 99, 235, 0.1);
  padding: 2px 8px;
  border-radius: 6px;
}

.invitation-item-card {
  border-top: 3px solid #f59e0b !important;
}

.member-neu-card {
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border: 1px solid var(--border-subtle);
  border-radius: 16px;
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 14px;
  min-width: 0;
  overflow: hidden;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.member-neu-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-flat);
}

.member-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.member-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.member-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  flex: 1;
}

.member-badges-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.role-pill {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
  box-shadow: var(--shadow-inset);
}

.role-pill.munimji {
  color: #3b82f6;
}

.role-pill.co_reviewer {
  color: #06b6d4;
}

.role-pill.participant {
  color: var(--text-muted);
}

.status-pill {
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
}

.status-pill.active {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.status-pill.invited {
  background: rgba(245, 158, 11, 0.15);
  color: #d97706;
}

.status-pill.rejected {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.status-pill.status-readonly {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
  text-align: center;
  padding: 6px 12px;
  border-radius: 10px;
}

.member-email {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
  margin-top: auto;
  min-height: 40px;
}

.member-join-date {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.member-actions-row {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.member-action-buttons {
  display: flex;
  gap: 8px;
  align-items: center;
}

.btn-pill-accept {
  flex: 1;
  background: #10b981;
  color: #ffffff;
  font-size: 11px;
  font-weight: 700;
  padding: 6px 12px;
  border-radius: 8px;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-pill-accept:hover {
  background: #059669;
}

.btn-pill-decline {
  flex: 1;
  background: transparent;
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
  font-size: 11px;
  font-weight: 700;
  padding: 6px 12px;
  border-radius: 8px;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-pill-decline:hover {
  background: rgba(239, 68, 68, 0.08);
}

/* Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  z-index: 100;
}

.modal-window {
  width: 100%;
  max-width: 480px;
  background-color: var(--surface);
  box-shadow: var(--shadow-flat-lg);
  border-radius: 22px;
  padding: 30px;
}

.modal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 22px;
}

.modal-headline {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.close-btn {
  border-radius: 50%;
  width: 30px;
  height: 30px;
  font-size: 12px;
}

.modal-form,
.prime-modal-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 4px;
}

.form-control {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.member-preview-box {
  padding: 10px 14px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  border-radius: 10px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.neu-field {
  background: var(--surface);
  border: 1px solid transparent;
  box-shadow: var(--shadow-inset);
  border-radius: 12px;
  padding: 12px 16px;
  color: var(--text-main);
  font-size: 14px;
  font-family: inherit;
  outline: none;
  transition: all 0.2s ease;
  width: 100%;
}

.neu-field:focus {
  box-shadow: var(--shadow-inset-focus);
  border-color: rgba(37, 99, 235, 0.3);
}

/* Custom Styled Select with Neumorphic Chevron */
select.neu-field {
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  background-image: url("data:image/svg+xml;charset=UTF-8,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%235e6673' stroke-width='2.5' stroke-linecap='round' stroke-linejoin='round'%3e%3cpolyline points='6 9 12 15 18 9'%3e%3c/polyline%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 14px center;
  background-size: 16px;
  padding-right: 42px;
  cursor: pointer;
}

select.neu-field option {
  background-color: var(--surface);
  color: var(--text-main);
  padding: 12px;
  font-size: 14px;
}

/* Eligible User Searchable Dropdown & Preview */
.eligible-user-picker {
  width: 100%;
}

.eligible-user-autocomplete {
  width: 100%;
}

.eligible-user-option {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 4px 2px;
}

.eligible-user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: linear-gradient(135deg, #0284c7, #06b6d4);
  color: #ffffff;
  font-size: 11px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  letter-spacing: 0.5px;
}

.eligible-user-meta {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.eligible-user-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.eligible-user-email {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.eligible-tag {
  font-size: 10px !important;
  padding: 2px 6px !important;
  flex-shrink: 0;
}

.eligible-empty-state {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  font-size: 13px;
  color: var(--text-muted);
}

.selected-user-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 12px;
  background: rgba(37, 99, 235, 0.08);
  border: 1px solid rgba(37, 99, 235, 0.22);
  margin-top: 2px;
  margin-bottom: 4px;
}

.selected-user-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #10b981, #059669);
  color: #ffffff;
  font-size: 12px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.selected-user-info {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.selected-user-name {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
}

.selected-user-email {
  font-size: 11px;
  color: var(--text-muted);
}

.input-hint-subtle {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 4px;
  line-height: 1.4;
}

.input-hint-subtle i {
  margin-top: 2px;
  font-size: 11px;
}


/* Polished Date-Time picker styling */
input[type="datetime-local"].neu-field,
input[type="date"].neu-field {
  cursor: pointer;
  color-scheme: light dark;
  position: relative;
  letter-spacing: -0.01em;
  font-family: 'Geist Mono', monospace;
  font-size: 13px;
}

input[type="datetime-local"].neu-field::-webkit-calendar-picker-indicator,
input[type="date"].neu-field::-webkit-calendar-picker-indicator {
  cursor: pointer;
  filter: invert(0.4);
  opacity: 0.8;
  transition: all 0.2s ease;
  padding: 4px;
}

html.dark input[type="datetime-local"].neu-field::-webkit-calendar-picker-indicator,
html.dark input[type="date"].neu-field::-webkit-calendar-picker-indicator {
  filter: invert(0.9);
}

input[type="datetime-local"].neu-field::-webkit-calendar-picker-indicator:hover,
input[type="date"].neu-field::-webkit-calendar-picker-indicator:hover {
  opacity: 1;
  transform: scale(1.1);
}

/* Toast Notifications Container */
.toast-stack {
  position: fixed;
  top: 24px;
  right: 24px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  z-index: 9999;
  pointer-events: none;
}

.neu-toast {
  pointer-events: auto;
  cursor: pointer;
  background: var(--surface);
  box-shadow: var(--shadow-flat-lg);
  border-radius: 12px;
  padding: 12px 18px;
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 280px;
  max-width: 420px;
  font-size: 13px;
  font-weight: 500;
  border-left: 4px solid var(--accent);
  animation: slideIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.neu-toast:hover {
  transform: translateY(-2px);
}

.neu-toast.success {
  border-left-color: #10b981;
}

.neu-toast.error {
  border-left-color: #ef4444;
}

.neu-toast.info {
  border-left-color: #3b82f6;
}

.toast-icon {
  font-size: 14px;
  font-weight: 700;
}

.neu-toast.success .toast-icon {
  color: #10b981;
}

.neu-toast.error .toast-icon {
  color: #ef4444;
}

.neu-toast.info .toast-icon {
  color: #3b82f6;
}

.toast-text {
  color: var(--text-main);
  line-height: 1.4;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(-12px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.form-row {
  display: flex;
  gap: 12px;
}

.modal-button-bar,
.dialog-actions-row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 12px;
}

.modal-cancel {
  border-radius: 10px;
  padding: 9px 18px;
}

.modal-submit {
  border-radius: 10px;
  padding: 9px 18px;
  color: var(--accent);
}

/* Mobile Drawer Top Bar */
.mobile-drawer-top-bar {
  display: none;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 14px 20px;
  border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.06));
  margin-bottom: 14px;
}

.mobile-drawer-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.small-badge {
  width: 28px !important;
  height: 28px !important;
  font-size: 13px !important;
}

.mobile-drawer-close {
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.06));
  color: var(--text-muted);
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mobile-drawer-close:hover {
  color: var(--text-main);
  transform: scale(1.05);
}

/* In-Page Responsive Workspace Tabs Bar */
.workspace-tabs-bar {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
  padding: 6px;
  background: var(--surface);
  box-shadow: var(--shadow-flat-sm);
  border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.05));
  border-radius: 14px;
  margin-bottom: 22px;
  position: sticky;
  top: 72px;
  z-index: 35;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  width: 100%;
}

.workspace-tab-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 9px 10px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.18s ease;
  font-family: inherit;
  width: 100%;
  text-align: center;
}

.workspace-tab-btn:hover {
  color: var(--text-main);
}

.workspace-tab-btn.active {
  background: var(--bg);
  color: var(--accent);
  box-shadow: var(--shadow-flat-sm);
  font-weight: 700;
}

.route-arrow-icon {
  font-size: 12px;
  color: var(--accent);
  opacity: 0.8;
  margin: 0 4px;
}


/* ==========================================================
   TRANSACTION LEDGER & ITEMIZED TABLES
   ========================================================== */
.ledger-records-section {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 30px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.records-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.records-subnav-pills {
  display: flex;
  gap: 10px;
  background: var(--surface);
  padding: 4px;
  border-radius: 12px;
  box-shadow: var(--shadow-inset);
}

.subnav-pill {
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  padding: 8px 16px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.subnav-pill.active {
  background: var(--surface);
  color: var(--text-main);
  box-shadow: var(--shadow-flat-sm);
}

.record-table-wrapper {
  overflow-x: auto;
  border-radius: 14px;
  box-shadow: var(--shadow-inset);
}

.neu-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 13px;
}

.neu-table th {
  padding: 14px 18px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

html.dark .neu-table th {
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.neu-table td {
  padding: 14px 18px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.03);
}

html.dark .neu-table td {
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

.neu-table tr:last-child td {
  border-bottom: none;
}

.method-tag {
  display: inline-block;
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(37, 99, 235, 0.1);
  color: var(--accent);
}

.amount-inflow {
  font-weight: 700;
  color: #10b981;
}

.amount-outflow {
  font-weight: 700;
  color: #ef4444;
}

.empty-records-msg {
  padding: 30px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  box-shadow: var(--shadow-inset);
  border-radius: 12px;
}

.text-right {
  text-align: right;
}

.font-bold {
  font-weight: 700;
}

/* Settlement Protocol Deck Container */
.settlement-protocol-deck {
  background-color: var(--surface);
  box-shadow: var(--shadow-flat);
  border-radius: 20px;
  padding: 26px 30px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  width: 100%;
}

.settlement-empty-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 48px 24px;
  background: var(--surface);
  border-radius: 20px;
  box-shadow: var(--shadow-flat-sm);
  border: 1px solid var(--border-subtle);
  margin-top: 10px;
  width: 100%;
  margin-left: auto;
  margin-right: auto;
}

.settlement-empty-icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: rgba(37, 99, 235, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 18px;
  box-shadow: var(--shadow-inset);
}

.settlement-empty-icon-wrap i {
  font-size: 28px;
  color: var(--accent);
}

.settlement-empty-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-main);
  margin: 0 0 10px 0;
}

.settlement-empty-desc {
  font-size: 13.5px;
  color: var(--text-muted);
  max-width: 480px;
  line-height: 1.55;
  margin: 0 0 24px 0;
}

.settlement-empty-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  justify-content: center;
}

.settlement-overview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.settlement-section-block {
  margin-top: 24px;
}

.sub-deck-title {
  font-size: 15px;
  font-weight: 700;
  margin-bottom: 12px;
  color: var(--text-main);
}

/* Clarification Notice Banner */
.settlement-clarification-banner {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
  background: var(--bg);
  border: 1px solid var(--border-subtle);
  border-radius: 14px;
  padding: 14px 16px;
  margin-bottom: 18px;
}

.clarification-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 12px;
  line-height: 1.45;
  color: var(--text-muted);
}

.clarification-item i {
  font-size: 14px;
  margin-top: 2px;
  flex-shrink: 0;
}

.clarification-item i.info-icon {
  color: #3b82f6;
}

.clarification-item i.success-icon {
  color: #10b981;
}

.clarification-item i.warn-icon {
  color: #f59e0b;
}

.clarification-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.clarification-title {
  font-weight: 700;
  color: var(--text-main);
  font-size: 12px;
}

/* Transfers Card Grid */
.transfers-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
  gap: 16px;
}

.transfer-neu-tile {
  background: var(--surface);
  border-radius: 16px;
  padding: 16px 18px;
  box-shadow: var(--shadow-flat);
  border: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 14px;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.transfer-neu-tile:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08);
}

.transfer-tile-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.transfer-type-banner {
  display: flex;
  align-items: center;
}

.transfer-amount-badge {
  font-size: 17px;
  font-weight: 800;
  color: var(--accent);
  background: rgba(37, 99, 235, 0.08);
  padding: 4px 10px;
  border-radius: 8px;
  border: 1px solid rgba(37, 99, 235, 0.16);
  white-space: nowrap;
}

.transfer-flow-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  background: var(--bg);
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--border-subtle);
}

.party-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
}

.party-badge {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.05em;
  padding: 2px 6px;
  border-radius: 4px;
  width: fit-content;
  white-space: nowrap;
}

.party-badge.debtor {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.party-badge.creditor {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.party-title-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.party-icon {
  font-size: 13px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.party-name {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
  white-space: normal;
  word-break: break-word;
  line-height: 1.3;
}

.transfer-arrow-divider {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--surface);
  border: 1px solid var(--border);
  flex-shrink: 0;
}

.flow-arrow-icon {
  color: var(--accent);
  font-size: 11px;
}

.sub-email {
  font-size: 11px;
  font-weight: 400;
  color: var(--text-muted);
  margin-left: 6px;
}

.section-title-wrap {
  margin-bottom: 12px;
}

.section-subtitle-hint {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
  margin-top: 4px;
}

.section-subtitle-hint i {
  margin-right: 4px;
  color: var(--accent);
}

.group-claims-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
}

.group-claim-card {
  background: var(--surface);
  border-radius: 16px;
  padding: 16px 18px;
  box-shadow: var(--shadow-flat);
  border: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
}

.group-claim-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
}

.claim-title-cluster {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.claim-item-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-main);
}

.claim-item-cat {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 700;
  color: var(--text-muted);
}

.claim-item-amount {
  font-size: 16px;
  font-weight: 800;
  color: var(--accent);
}

.group-claim-body {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.claim-claimant-info,
.claim-debtor-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.group-claim-footer {
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px dashed var(--border-subtle);
}

.transfer-type-banner {
  margin-bottom: 10px;
}

.transfer-desc-footer {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
  font-size: 11px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.net-position-cluster {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.net-pills-row {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.sub-breakdown-pill {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 6px;
  white-space: nowrap;
}

.claim-pill {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}

.refund-pill {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.dues-pill {
  background: rgba(6, 182, 212, 0.12);
  color: #06b6d4;
}

.owes-pill {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

/* Status Controller & Actions Cluster */
.hero-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.hero-actions-cluster {
  display: flex;
  align-items: flex-end;
  gap: 10px;
}

.status-control-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-select-component {
  background: var(--surface) !important;
  box-shadow: var(--shadow-inset) !important;
  border: 1px solid var(--border, rgba(125, 133, 146, 0.2)) !important;
  border-radius: 10px !important;
  min-width: 145px;
  height: 36px;
  display: flex;
  align-items: center;
}

.status-select-component :deep(.p-select-label) {
  padding: 4px 10px !important;
  font-size: 12px;
}

.status-select-component :deep(.p-select-dropdown) {
  width: 28px;
}

/* Modal Form PrimeVue Select styling (consistent with .neu-field) */
.neu-select-prime {
  background: var(--surface) !important;
  box-shadow: var(--shadow-inset) !important;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08)) !important;
  border-radius: 12px !important;
  height: 42px;
  display: flex;
  align-items: center;
  transition: all 0.2s ease;
}

.neu-select-prime:focus-within {
  border-color: var(--accent) !important;
  box-shadow: var(--shadow-inset), 0 0 0 1px var(--accent) !important;
}

.neu-select-prime :deep(.p-select-label) {
  padding: 8px 14px !important;
  font-size: 13px !important;
  color: var(--text-main) !important;
  font-weight: 500;
}

.neu-select-prime :deep(.p-select-dropdown) {
  width: 34px;
  color: var(--text-muted);
}

.btn-delete-expedition {
  height: 36px !important;
  padding: 0 12px !important;
  border-radius: 10px !important;
  font-size: 11px !important;
  font-weight: 600 !important;
  background: var(--surface) !important;
  box-shadow: var(--shadow-flat-sm) !important;
  border: 1px solid rgba(239, 68, 68, 0.3) !important;
  color: #ef4444 !important;
  transition: all 0.2s ease !important;
}

.btn-delete-expedition:hover {
  transform: translateY(-1px) !important;
  border-color: #ef4444 !important;
  background: rgba(239, 68, 68, 0.08) !important;
}

.btn-delete-expedition:active {
  transform: translateY(1px) !important;
  box-shadow: var(--shadow-inset) !important;
}

/* Receipt Preview & Rejection Note */
.receipt-view-link {
  font-size: 11px;
  font-weight: 700;
  color: var(--accent);
  text-decoration: none;
  background: var(--accent-bg, rgba(6, 182, 212, 0.12));
  border: 1px solid var(--accent-border, rgba(6, 182, 212, 0.25));
  padding: 3px 10px;
  border-radius: 6px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-family: inherit;
  transition: all 0.2s ease;
}

.receipt-view-link:hover {
  background: var(--accent);
  color: #fff;
  transform: translateY(-1px);
}

.receipt-view-link:active {
  transform: translateY(1px);
}

.receipt-modal-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.receipt-meta-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-radius: 10px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
}

.receipt-content-box {
  display: flex;
  justify-content: center;
  align-items: center;
  background: rgba(0, 0, 0, 0.04);
  border: 1px dashed var(--border-subtle, rgba(0, 0, 0, 0.12));
  border-radius: 12px;
  overflow: hidden;
  max-height: 65vh;
  padding: 12px;
}

.receipt-image-preview {
  max-width: 100%;
  max-height: 58vh;
  object-fit: contain;
  border-radius: 8px;
  box-shadow: var(--shadow-flat-sm);
}

.receipt-pdf-frame {
  width: 100%;
  height: 58vh;
  border: none;
  border-radius: 8px;
}

.receipt-external-btn {
  display: inline-flex;
  align-items: center;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 600;
  border-radius: 8px;
  background: var(--accent);
  color: #ffffff !important;
  text-decoration: none;
  box-shadow: 0 2px 8px rgba(6, 182, 212, 0.3);
  transition: all 0.2s ease;
}

.receipt-external-btn:hover {
  filter: brightness(1.08);
}

.rejection-note {
  font-size: 11px;
  color: #ef4444;
  margin-top: 4px;
  font-style: italic;
}

.file-hint {
  font-size: 11px;
  color: var(--accent);
  font-weight: 600;
  margin-top: 4px;
}

.text-area-field {
  resize: vertical;
  min-height: 80px;
}

.modal-desc-text {
  font-size: 13px;
  color: var(--text-muted);
  margin: 0 0 14px 0;
  line-height: 1.4;
}

/* Sidebar Invitations Group */
.sidebar-invitations-group {
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px dashed var(--border);
}

.sidebar-invitations-scroll {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 10px;
}

.sidebar-invitation-card {
  background: rgba(37, 99, 235, 0.04);
  border: 1px solid rgba(37, 99, 235, 0.2);
  border-radius: 12px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: all 0.2s ease;
}

.sidebar-invitation-card:hover {
  background: rgba(37, 99, 235, 0.08);
  border-color: rgba(37, 99, 235, 0.35);
}

.sidebar-inv-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.sidebar-inv-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-inv-route {
  font-size: 11px;
  color: var(--text-muted);
}

.sidebar-inv-actions {
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

.sidebar-empty-trips {
  padding: 16px;
  font-size: 12px;
  color: var(--text-muted);
  text-align: center;
  font-style: italic;
}

/* Invited Decision Hero Deck */
.invited-decision-deck {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 48px 20px;
}

.invited-decision-card {
  max-width: 600px;
  width: 100%;
  background: var(--surface);
  border: 1px solid rgba(125, 133, 146, 0.2);
  border-radius: 24px;
  padding: 44px 36px;
  text-align: center;
  box-shadow: var(--shadow-flat);
}

.invited-icon-badge {
  width: 68px;
  height: 68px;
  border-radius: 20px;
  background: var(--accent-muted, rgba(37, 99, 235, 0.15));
  color: var(--accent);
  box-shadow: var(--shadow-flat-sm);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 30px;
  margin-bottom: 20px;
}

.invited-pill {
  display: inline-block;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.8px;
  text-transform: uppercase;
  color: var(--accent);
  background: var(--accent-muted, rgba(37, 99, 235, 0.12));
  border: 1px solid rgba(37, 99, 235, 0.25);
  padding: 5px 14px;
  border-radius: 20px;
  margin-bottom: 14px;
}

.invited-title {
  font-size: 26px;
  font-weight: 800;
  color: var(--text-main);
  margin: 0 0 12px 0;
  letter-spacing: -0.3px;
}

.invited-desc {
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-muted);
  margin: 0 0 28px 0;
}

.invited-trip-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: var(--active-pill, var(--bg));
  box-shadow: var(--shadow-inset);
  border: 1px solid rgba(125, 133, 146, 0.2);
  border-radius: 16px;
  padding: 18px 22px;
  margin-bottom: 28px;
  text-align: left;
}

.summary-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}

.summary-label {
  color: var(--text-muted);
  font-weight: 500;
}

.summary-val {
  color: var(--text-main);
}

.role-highlight {
  font-weight: 700;
  color: var(--accent);
}

.invited-actions-row {
  display: flex;
  justify-content: center;
  gap: 14px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.invited-fineprint {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
  margin: 0;
}

/* Empty Workspace Pending Invitations Deck */
.pending-invites-hero-deck {
  width: 100%;
  max-width: 800px;
  margin: 0 auto;
  padding: 32px 16px;
}

.pending-invites-hero-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 28px;
}

.pending-invites-hero-header .hero-icon-badge {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: rgba(37, 99, 235, 0.1);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
  margin-bottom: 0;
}

.pending-invites-hero-header .hero-title {
  font-size: 22px;
  font-weight: 800;
  color: var(--text-main);
  margin: 0 0 4px 0;
}

.pending-invites-hero-header .hero-sub {
  font-size: 13px;
  color: var(--text-muted);
  margin: 0;
}

.pending-invitations-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 18px;
}

.pending-invitation-hero-card {
  background: var(--card-bg, #ffffff);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 22px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.03);
  display: flex;
  flex-direction: column;
  gap: 16px;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.pending-invitation-hero-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.06);
}

.hero-card-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.hero-card-title-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-trip-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-main);
}

.hero-route {
  font-size: 12px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.hero-inv-details {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 14px;
  background: var(--bg);
  border-radius: 10px;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.detail-label {
  color: var(--text-muted);
}

.detail-val {
  color: var(--text-main);
}

.hero-card-actions {
  display: flex;
  gap: 10px;
}

.plan-alternative-box {
  text-align: center;
}

/* ==========================================================
   PRODUCTION-GRADE RESPONSIVE DESIGN SYSTEM
   Supports: 4K/Ultrawide, Desktop, Tablet, Large/Compact Mobile
   ========================================================== */

/* 1. Large Screens & Laptops (<= 1200px) */
@media (max-width: 1200px) {
  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }
  .action-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }
}

/* 2. Tablets & Small Laptops (<= 960px) */
@media (max-width: 960px) {
  .app-header {
    padding: 0 18px;
  }
  .header-caption, .brand-divider {
    display: none;
  }
  .mobile-menu-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .app-content-layout {
    padding: 14px;
    gap: 14px;
  }

  /* Full-height Native Slide-out Sidebar Drawer */
  .sidebar-column {
    position: fixed;
    top: 0;
    left: -340px;
    bottom: 0;
    width: min(85vw, 320px);
    height: 100vh;
    max-height: 100vh;
    z-index: 1000;
    border-radius: 0 22px 22px 0;
    box-shadow: 10px 0 40px rgba(0, 0, 0, 0.25);
    transition: left 0.28s cubic-bezier(0.16, 1, 0.3, 1);
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
  }

  .sidebar-column.drawer-open {
    left: 0;
  }

  .mobile-drawer-top-bar {
    display: flex;
  }

  .mobile-drawer-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    z-index: 999;
  }

  .trip-hero-section {
    flex-direction: column;
    align-items: stretch;
    padding: 20px 18px;
    gap: 16px;
  }

  .hero-right {
    display: flex;
    flex-direction: row;
    flex-wrap: wrap;
    gap: 10px;
    width: 100%;
  }

  .status-control-box,
  .treasurer-chip {
    flex: 1;
    min-width: 150px;
    align-items: flex-start;
  }
}

/* 3. Small Tablets & Landscape Mobile (<= 768px) */
@media (max-width: 768px) {
  .app-header {
    padding: 0 14px;
    height: 60px;
  }

  .user-email-text {
    display: none;
  }

  .user-profile-btn {
    padding: 4px 6px 4px 4px;
    gap: 4px;
  }

  .header-caption {
    display: none;
  }

  .brand-divider {
    display: none;
  }

  .header-trip-pill {
    max-width: 100px;
    font-size: 11px;
  }

  /* Make Plan Trip button a clean circular icon-only button on mobile */
  .btn-plan-header :deep(.p-button-label) {
    display: none !important;
  }

  .btn-plan-header {
    width: 36px !important;
    min-width: 36px !important;
    height: 36px !important;
    padding: 0 !important;
    border-radius: 50% !important;
    display: inline-flex !important;
    align-items: center !important;
    justify-content: center !important;
  }

  .btn-plan-header :deep(.p-button-icon) {
    margin: 0 !important;
    font-size: 14px !important;
  }

  .workspace-tabs-bar {
    top: 66px;
    padding: 4px;
    border-radius: 12px;
    margin-bottom: 16px;
    gap: 4px;
    grid-template-columns: repeat(4, 1fr);
  }

  .workspace-tab-btn {
    padding: 7px 4px;
    font-size: 11px;
    gap: 4px;
    flex-direction: column;
  }

  .workspace-tab-btn i {
    font-size: 13px;
  }

  .workspace-tab-btn span {
    font-size: 10px;
    line-height: 1.1;
  }

  .hero-title {
    font-size: 22px;
  }

  .hero-route-line {
    flex-wrap: wrap;
    gap: 6px;
  }

  .claim-neu-card {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
    padding: 16px;
  }

  .claim-card-right {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    border-top: 1px solid var(--border-subtle);
    padding-top: 12px;
    margin-top: 2px;
  }

  .claim-amount {
    font-size: 18px;
  }

  .deck-header-row,
  .records-header-row {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .records-subnav-pills {
    width: 100%;
  }

  .subnav-pill {
    flex: 1;
    text-align: center;
    font-size: 12px;
    padding: 8px 10px;
  }

  .operations-deck,
  .claims-review-deck,
  .members-governance-deck,
  .settlement-protocol-deck,
  .ledger-records-section {
    padding: 20px 16px;
    border-radius: 16px;
  }

  .members-grid-stack {
    grid-template-columns: 1fr;
  }

  .member-neu-card {
    padding: 16px;
  }

  .transfers-card-grid {
    grid-template-columns: 1fr;
  }

  .settlement-overview-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
}

/* 4. Standard Mobile Phones (<= 600px) */
@media (max-width: 600px) {
  .metrics-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .metric-card {
    padding: 16px;
    border-radius: 14px;
  }

  .metric-number {
    font-size: 22px;
  }

  .action-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .action-tile {
    padding: 16px;
  }

  .tile-title {
    font-size: 14px;
  }

  .tile-text {
    margin-bottom: 14px;
  }

  .form-row {
    flex-direction: column;
    gap: 12px;
  }

  .record-table-wrapper {
    margin: 0 -4px;
  }

  .neu-table th,
  .neu-table td {
    padding: 10px 8px;
    font-size: 11px;
    white-space: nowrap;
  }

  .pending-invites-hero-deck {
    padding: 16px 8px;
  }

  .pending-invites-hero-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .pending-invitations-cards-grid {
    grid-template-columns: 1fr;
    gap: 14px;
  }

  .pending-invitation-hero-card {
    padding: 16px;
  }

  .hero-card-actions {
    flex-direction: column;
    gap: 8px;
  }

  .hero-card-actions button {
    width: 100%;
  }

  .invited-decision-deck {
    padding: 24px 10px;
  }

  .invited-decision-card {
    padding: 28px 16px;
    border-radius: 18px;
  }

  .invited-title {
    font-size: 20px;
  }

  .invited-actions-row {
    flex-direction: column;
    gap: 10px;
    width: 100%;
  }

  .invited-actions-row button {
    width: 100%;
  }

  .transfer-flow-container {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .transfer-arrow-divider {
    align-self: center;
    transform: rotate(90deg);
  }

  /* PrimeVue Dialog Mobile Optimization */
  .p-dialog {
    width: 95vw !important;
    max-width: 95vw !important;
    margin: 0 auto !important;
  }

  .p-dialog .p-dialog-header {
    padding: 18px 18px 12px 18px !important;
  }

  .p-dialog .p-dialog-content {
    padding: 0 18px 18px 18px !important;
  }

  .dialog-actions-row {
    flex-direction: column-reverse;
    gap: 8px;
    width: 100%;
  }

  .dialog-actions-row button {
    width: 100%;
  }
}

/* 5. Extra Compact Mobile Screens (<= 440px, e.g. iPhone SE, Fold) */
@media (max-width: 440px) {
  .app-header {
    padding: 0 10px;
  }

  .brand-title {
    font-size: 17px;
  }

  .header-trip-pill {
    display: none;
  }

  .route-node {
    font-size: 11px;
    padding: 2px 7px;
  }

  .hero-title {
    font-size: 19px;
  }

  .claim-action-buttons {
    flex-direction: column;
    gap: 6px;
    width: 100%;
  }

  .claim-action-buttons button {
    width: 100%;
  }

  .claim-card-right {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .transfer-from-to {
    flex-direction: column;
    gap: 10px;
    text-align: center;
    align-items: center;
  }

  .transfer-arrow-block {
    transform: rotate(90deg);
    margin: 4px 0;
  }
}

.trip-stops-editor,
.route-map-preview {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 10px;
}

.trip-stops-heading,
.route-map-preview-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.trip-stops-heading > div,
.route-map-preview-heading > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.wizard-section-box {
  padding: 16px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.015);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.05));
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.wizard-section-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
  text-transform: uppercase;
  letter-spacing: 0.6px;
  display: flex;
  align-items: center;
}

.date-validation-error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  padding: 8px 12px;
  border-radius: 10px;
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #f87171;
  font-size: 12px;
  font-weight: 600;
  animation: fadeIn 0.2s ease;
}

.trip-duration-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  padding: 8px 12px;
  border-radius: 10px;
  background: rgba(56, 189, 248, 0.08);
  border: 1px solid rgba(56, 189, 248, 0.25);
  color: var(--accent, #38bdf8);
  font-size: 12px;
  font-weight: 600;
  animation: fadeIn 0.2s ease;
}

.input-has-error {
  border-color: #ef4444 !important;
  box-shadow: var(--shadow-inset), 0 0 0 1px #ef4444 !important;
}

.trip-stops-empty-hint {
  padding: 12px 14px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px dashed var(--border-subtle, rgba(255, 255, 255, 0.1));
  color: var(--text-muted);
  font-size: 12px;
  text-align: center;
}

.trip-stops-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 8px;
}

.trip-stop-row {
  display: grid;
  grid-template-columns: 32px 32px minmax(0, 1fr) 36px;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.025);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  border-radius: 16px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.18);
  transition: all 0.22s cubic-bezier(0.16, 1, 0.3, 1);
}

.trip-stop-row:hover {
  background: rgba(255, 255, 255, 0.045);
  border-color: rgba(56, 189, 248, 0.3);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25);
  transform: translateY(-1px);
}

.trip-stop-row.is-dragging {
  opacity: 0.45;
  border-style: dashed;
  border-color: var(--accent);
  transform: scale(0.98);
  background: rgba(56, 189, 248, 0.05);
}

.trip-stop-row.is-drag-over {
  border-color: var(--accent);
  background: rgba(56, 189, 248, 0.1);
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.3), 0 8px 24px rgba(0, 0, 0, 0.35);
}

.trip-stop-drag-handle {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  color: var(--text-muted);
  cursor: grab;
  font-size: 14px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  transition: all 0.18s ease;
  user-select: none;
}

.trip-stop-drag-handle:hover {
  color: var(--accent);
  background: rgba(56, 189, 248, 0.15);
}

.trip-stop-drag-handle:active {
  cursor: grabbing;
}

.trip-stop-order {
  width: 32px;
  height: 32px;
  min-width: 32px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.4);
  color: var(--accent, #38bdf8);
  font-size: 13px;
  font-weight: 800;
  user-select: none;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.12);
}

.route-expedition-box {
  padding: 22px !important;
  border-radius: 18px !important;
  gap: 18px !important;
}

.route-section-heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.07));
}

.route-section-title {
  font-size: 14px;
  font-weight: 800;
  color: var(--text-main);
  text-transform: uppercase;
  letter-spacing: 0.8px;
  display: flex;
  align-items: center;
}

.route-section-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.route-milestone-card {
  padding: 16px 18px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  display: flex;
  flex-direction: column;
  gap: 10px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.14);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.route-milestone-card:hover {
  border-color: rgba(255, 255, 255, 0.14);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.22);
}

.start-milestone-card {
  border-left: 3px solid #10b981;
}

.destination-milestone-card {
  border-left: 3px solid #f43f5e;
}

.milestone-badge-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.milestone-pill {
  display: inline-flex;
  align-items: center;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.7px;
  padding: 4px 10px;
  border-radius: 8px;
  text-transform: uppercase;
}

.start-pill {
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.dest-pill {
  color: #f43f5e;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
}

.milestone-role-hint {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.milestone-subtext {
  font-size: 11px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
}

.stops-section-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-main);
  display: flex;
  align-items: center;
}

.trip-stops-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.015);
  border: 1px dashed rgba(255, 255, 255, 0.09);
  margin: 4px 0;
}

.trip-stop-input-wrap {
  min-width: 0;
  width: 100%;
}

.trip-stop-actions {
  display: flex;
  align-items: center;
  justify-content: center;
}

.trip-stop-delete-btn {
  width: 34px !important;
  height: 34px !important;
  color: var(--text-muted) !important;
  border-radius: 10px !important;
  transition: all 0.15s ease !important;
}

.trip-stop-delete-btn:hover {
  color: #ef4444 !important;
  background: rgba(239, 68, 68, 0.15) !important;
}

.total-distance-card {
  margin-top: 12px;
  padding: 16px 20px;
  border-radius: 16px;
  background: var(--surface);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  box-shadow: var(--shadow-flat-sm);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.total-distance-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.total-distance-badge-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.total-distance-icon-glow {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: rgba(56, 189, 248, 0.14);
  color: var(--accent, #38bdf8);
  display: grid;
  place-items: center;
  font-size: 19px;
  border: 1px solid rgba(56, 189, 248, 0.25);
  box-shadow: 0 0 16px rgba(56, 189, 248, 0.15);
}

.total-distance-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  color: var(--text-muted);
}

.total-distance-value {
  font-size: 24px;
  font-weight: 800;
  color: var(--text-main);
  letter-spacing: -0.5px;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.total-distance-loading,
.total-distance-direct {
  font-size: 13px;
  color: var(--text-muted);
}

.legs-count-pill {
  padding: 4px 12px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  color: var(--text-main);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

.total-distance-legs-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.06));
}

.distance-leg-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  padding: 6px 10px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
  gap: 8px;
}

.leg-left {
  display: flex;
  align-items: center;
  min-width: 0;
  overflow: hidden;
}

.leg-index {
  width: 20px;
  height: 20px;
  min-width: 20px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  display: grid;
  place-items: center;
  font-size: 10px;
  font-weight: 700;
  margin-right: 8px;
}

.leg-names {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.leg-names strong {
  font-weight: 600;
  color: var(--text-main);
}

.leg-arrow {
  font-size: 10px;
  color: var(--accent);
}

.leg-km {
  font-weight: 700;
  color: var(--accent, #38bdf8);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.trip-stop-controls {
  display: flex;
  justify-content: flex-end;
  gap: 0;
}



.route-map-preview {
  padding-top: 14px;
  border-top: 1px solid var(--border-subtle, rgba(125, 133, 146, 0.2));
}

.route-distance-note,
.route-map-empty {
  color: var(--text-muted);
  font-size: 11px;
}

.route-map-empty {
  margin: 0;
  padding: 20px 14px;
  border-radius: 12px;
  background: var(--surface);
  box-shadow: var(--shadow-inset);
  text-align: center;
}

.pass-stop-list {
  margin: -6px 0 16px;
  color: var(--text-muted);
  font-size: 10px;
  line-height: 1.5;
  text-align: center;
}

@media (max-width: 768px) {
  .trip-stops-heading,
  .route-map-preview-heading {
    align-items: flex-start;
  }

  .route-distance-note {
    text-align: right;
  }
}

/* ==========================================================================
   Comprehensive Mobile & Tablet Responsive Optimizations (Trip Wizard & Dashboard)
   ========================================================================== */
@media (max-width: 768px) {
  .app-content-layout {
    padding: 12px 10px !important;
    gap: 12px !important;
  }

  .wizard-split-layout {
    grid-template-columns: 1fr !important;
    width: 100% !important;
    max-width: 100% !important;
  }

  .wizard-surface-card {
    padding: 18px 14px !important;
    border-radius: 16px !important;
    gap: 16px !important;
  }

  .wizard-section-box {
    padding: 14px 12px !important;
    border-radius: 14px !important;
    gap: 12px !important;
  }

  .route-expedition-box {
    padding: 14px 12px !important;
    border-radius: 14px !important;
    gap: 14px !important;
  }

  .route-section-heading {
    padding-bottom: 10px !important;
    margin-bottom: 2px !important;
  }

  .route-section-title {
    font-size: 13px !important;
    letter-spacing: 0.6px !important;
  }

  .route-section-subtitle {
    font-size: 11px !important;
    line-height: 1.35 !important;
  }

  .route-milestone-card {
    padding: 12px 12px !important;
    border-radius: 12px !important;
    gap: 8px !important;
  }

  .milestone-badge-row {
    flex-wrap: wrap !important;
    gap: 6px !important;
  }

  .milestone-pill {
    font-size: 10px !important;
    padding: 3px 8px !important;
  }

  .milestone-role-hint {
    font-size: 11px !important;
  }

  .milestone-subtext {
    font-size: 10px !important;
    line-height: 1.35 !important;
  }

  .trip-stops-editor {
    padding: 12px 10px !important;
    border-radius: 12px !important;
    gap: 10px !important;
    margin: 2px 0 !important;
  }

  .trip-stops-heading {
    flex-direction: row !important;
    align-items: center !important;
    justify-content: space-between !important;
    gap: 8px !important;
  }

  .stops-section-title {
    font-size: 12px !important;
  }

  .trip-stops-list {
    gap: 8px !important;
  }

  .trip-stop-row {
    grid-template-columns: 28px 26px minmax(0, 1fr) 30px !important;
    gap: 8px !important;
    padding: 8px 10px !important;
    border-radius: 12px !important;
  }

  .trip-stop-drag-handle {
    width: 28px !important;
    height: 28px !important;
    font-size: 12px !important;
    border-radius: 8px !important;
    touch-action: none !important;
  }

  .trip-stop-order {
    width: 26px !important;
    height: 26px !important;
    min-width: 26px !important;
    font-size: 11px !important;
    border-radius: 8px !important;
  }

  .trip-stop-delete-btn {
    width: 30px !important;
    height: 30px !important;
    font-size: 12px !important;
    border-radius: 8px !important;
  }

  .total-distance-card {
    padding: 12px 14px !important;
    border-radius: 14px !important;
    gap: 10px !important;
  }

  .total-distance-header {
    gap: 8px !important;
  }

  .total-distance-badge-wrap {
    gap: 10px !important;
  }

  .total-distance-icon-glow {
    width: 36px !important;
    height: 36px !important;
    font-size: 16px !important;
    border-radius: 10px !important;
  }

  .total-distance-label {
    font-size: 10px !important;
  }

  .total-distance-value {
    font-size: 20px !important;
  }

  .total-distance-legs-list {
    gap: 8px !important;
  }

  .distance-leg-item {
    flex-direction: column !important;
    align-items: flex-start !important;
    gap: 6px !important;
    padding: 8px 10px !important;
    border-radius: 8px !important;
  }

  .leg-left {
    width: 100% !important;
    align-items: flex-start !important;
  }

  .leg-names {
    flex-wrap: wrap !important;
    white-space: normal !important;
    word-break: break-word !important;
    font-size: 11px !important;
    line-height: 1.35 !important;
  }

  .leg-km {
    align-self: flex-end !important;
    font-size: 12px !important;
  }

  .wizard-two-col {
    grid-template-columns: 1fr !important;
    gap: 12px !important;
  }

  .wizard-two-col input[type="datetime-local"] {
    width: 100% !important;
    box-sizing: border-box !important;
    min-height: 42px !important;
    font-size: 13px !important;
    padding: 9px 12px !important;
  }

  .wizard-actions-bar {
    flex-direction: column-reverse !important;
    gap: 10px !important;
    padding-top: 14px !important;
    margin-top: 6px !important;
  }

  .wizard-actions-bar button {
    width: 100% !important;
    justify-content: center !important;
  }

  .btn-wizard-next,
  .btn-wizard-launch {
    min-height: 46px !important;
    font-size: 14px !important;
    font-weight: 700 !important;
  }

  .btn-wizard-back-small {
    min-height: 38px !important;
  }

  .summary-details-grid {
    grid-template-columns: 1fr !important;
    gap: 10px !important;
  }

  .summary-col {
    padding: 8px 10px !important;
    background: rgba(255, 255, 255, 0.02) !important;
    border-radius: 8px !important;
  }

  .summary-val {
    word-break: break-word !important;
  }
}

/* Extra Compact Mobile Screens (<= 480px, e.g. iPhone SE, Fold) */
@media (max-width: 480px) {
  .app-content-layout {
    padding: 8px 6px !important;
    gap: 8px !important;
  }

  .wizard-surface-card {
    padding: 12px 10px !important;
    border-radius: 14px !important;
    gap: 12px !important;
  }

  .wizard-stepper {
    padding: 5px 6px !important;
    gap: 2px !important;
  }

  .stepper-node {
    padding: 4px 4px !important;
    gap: 4px !important;
  }

  .stepper-node:not(.node-active) .node-title {
    display: none !important;
  }

  .stepper-node.node-active {
    flex: 2 !important;
    background: rgba(56, 189, 248, 0.08) !important;
    border-radius: 8px !important;
  }

  .stepper-node.node-active .node-title {
    font-size: 11px !important;
    font-weight: 700 !important;
  }

  .node-circle {
    width: 22px !important;
    height: 22px !important;
    min-width: 22px !important;
    font-size: 10px !important;
  }

  .step-intro {
    gap: 10px !important;
  }

  .step-icon-badge {
    width: 36px !important;
    height: 36px !important;
    min-width: 36px !important;
    font-size: 15px !important;
  }

  .step-heading {
    font-size: 16px !important;
    line-height: 1.25 !important;
  }

  .step-subheading {
    font-size: 11px !important;
  }

  .wizard-section-box {
    padding: 10px 8px !important;
    border-radius: 12px !important;
    gap: 10px !important;
  }

  .route-expedition-box {
    padding: 10px 8px !important;
    border-radius: 12px !important;
    gap: 10px !important;
  }

  .route-milestone-card {
    padding: 10px 8px !important;
    border-radius: 10px !important;
    gap: 6px !important;
  }

  .trip-stops-editor {
    padding: 8px 6px !important;
    border-radius: 10px !important;
    gap: 8px !important;
  }

  .trip-stop-row {
    grid-template-columns: 24px 22px minmax(0, 1fr) 28px !important;
    gap: 6px !important;
    padding: 6px 8px !important;
    border-radius: 10px !important;
  }

  .trip-stop-drag-handle {
    width: 24px !important;
    height: 24px !important;
    font-size: 11px !important;
    border-radius: 6px !important;
  }

  .trip-stop-order {
    width: 22px !important;
    height: 22px !important;
    min-width: 22px !important;
    font-size: 10px !important;
    border-radius: 6px !important;
  }

  .trip-stop-delete-btn {
    width: 28px !important;
    height: 28px !important;
    font-size: 11px !important;
    border-radius: 6px !important;
  }
}
</style>
