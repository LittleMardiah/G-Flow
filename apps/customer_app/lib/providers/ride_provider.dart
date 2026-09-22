import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:latlong2/latlong.dart';

import '../models/ride_order.dart';
import '../services/ride_service.dart';
import 'auth_provider.dart';

/// Provider layanan ride. Memakai ApiClient yang sudah ada (Dio + JWT).
final rideServiceProvider = Provider((ref) => RideService(ref.watch(apiClientProvider)));

/// State traking ride untuk satu order.
class RideTrackingState {
  const RideTrackingState({
    this.isLoading = true,
    this.isMock = false,
    this.error,
    this.order,
    this.driver,
    this.driverLocation,
    this.isCancelling = false,
    this.message,
  });

  final bool isLoading;
  final bool isMock;
  final String? error;
  final RideOrder? order;
  final DriverInfo? driver;
  final LatLng? driverLocation;
  final bool isCancelling;
  final String? message;

  RideTrackingState copyWith({
    bool? isLoading,
    bool? isMock,
    String? error,
    RideOrder? order,
    DriverInfo? driver,
    LatLng? driverLocation,
    bool? isCancelling,
    String? message,
  }) {
    return RideTrackingState(
      isLoading: isLoading ?? this.isLoading,
      isMock: isMock ?? this.isMock,
      error: error ?? this.error,
      order: order ?? this.order,
      driver: driver ?? this.driver,
      driverLocation: driverLocation ?? this.driverLocation,
      isCancelling: isCancelling ?? this.isCancelling,
      message: message ?? this.message,
    );
  }
}

/// Notifier tracking: polling status ride setiap 3 detik (ROADMAP 02 bagian
/// 3.2 — "Order status polling"). Saat order mencapai status terminal
/// (COMPLETED/SETTLED/CANCELLED) polling dihentikan otomatis.
class RideTrackingNotifier extends StateNotifier<RideTrackingState> {
  RideTrackingNotifier(this._service, this._args)
      : super(RideTrackingState(isLoading: true, isMock: kUseMockRideData));

  final RideService _service;
  final RideTrackingArgs _args;

  Timer? _timer;
  int _tick = 0;
  bool _polling = false;

  static const Duration pollInterval = Duration(seconds: 3);

  void start() {
    if (_polling) return;
    _polling = true;
    _fetch();
    _timer = Timer.periodic(pollInterval, (_) => _fetch());
  }

  void stopPolling() {
    _polling = false;
    _timer?.cancel();
    _timer = null;
  }

  /// Dipanggil tombol "Coba lagi" pada error view.
  Future<void> retry() => _fetch();

  Future<void> _fetch() async {
    if (!_polling) return;
    if (state.order?.isTerminal == true) {
      stopPolling();
      return;
    }

    if (kUseMockRideData) {
      _applyMockStep();
      return;
    }

    try {
      // TODO: setelah backend mengembalikan info driver & lokasi driver,
      // tambahkan pemanggilan definitf di sini (order + driver + lokasi).
      final order = await _service.getRideDetails(_args.orderId);
      state = RideTrackingState(isLoading: false, isMock: false, order: order);
      if (order.isTerminal) stopPolling();
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat status ride (jaringan/API). Pastikan server aktif.',
      );
    }
  }

  void _applyMockStep() {
    _tick++;
    final status = RideMockSimulator.statusForTick(_tick);
    final driver = status == 'SEARCHING_DRIVER'
        ? null
        : RideMockSimulator.driverForTick(_tick);
    final driverLocation = driver == null
        ? null
        : RideMockSimulator.driverLocationForTick(
            pickup: LatLng(_args.pickupLat, _args.pickupLng),
            dropoff: LatLng(_args.dropoffLat, _args.dropoffLng),
            tick: _tick,
          );

    // Basis order memakai `_seed` dari RideTrackingArgs karena GET detail
    // order belum tersedia backend (mock). Bilah proyeksi koordinat pickup/
    // dropoff pun berasal dari sini.
    final order = RideOrder(
      id: _args.orderId,
      driverId: driver?.id,
      status: status,
      pickupLat: _args.pickupLat,
      pickupLng: _args.pickupLng,
      dropoffLat: _args.dropoffLat,
      dropoffLng: _args.dropoffLng,
      pickupAddress: _args.pickupAddress,
      dropoffAddress: _args.dropoffAddress,
      distanceKm: _args.distanceKm,
      estimatedFare: _args.estimatedFare,
      paymentMethod: _args.paymentMethod,
    );

    state = RideTrackingState(
      isLoading: false,
      isMock: true,
      order: order,
      driver: driver,
      driverLocation: driverLocation,
    );
    if (order.isTerminal) stopPolling();
  }

  /// Membatalkan ride. Mengembalikan true jika sukses. Pada mode mock,
  /// request hanya disimulasikan (delay singkat) lalu status di-set CANCELLED.
  Future<bool> cancelRide() async {
    if (state.isCancelling || state.order?.isTerminal == true) return false;
    state = state.copyWith(isCancelling: true, error: null);

    try {
      if (!kUseMockRideData) {
        await _service.cancelRide(_args.orderId);
      } else {
        await Future<void>.delayed(const Duration(milliseconds: 800));
      }

      final current = state.order;
      state = RideTrackingState(
        isLoading: false,
        isMock: kUseMockRideData,
        order: current?.copyWith(status: 'CANCELLED', cancellationReason: 'CUSTOMER_CANCEL'),
        message: 'Ride berhasil dibatalkan',
      );
      stopPolling();
      return true;
    } catch (_) {
      state = state.copyWith(
        isCancelling: false,
        error: 'Gagal membatalkan ride. Periksa koneksi lalu coba lagi.',
      );
      return false;
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}

/// Provider tracking per order (family). Dibuat saat tracking screen pertama
/// kali mem-watch provider ini, polling berjalan otomatis via start().
final rideTrackingProvider =
    StateNotifierProvider.family<RideTrackingNotifier, RideTrackingState, RideTrackingArgs>(
  (ref, args) {
    final notifier = RideTrackingNotifier(ref.watch(rideServiceProvider), args);
    return notifier..start();
  },
);

/// State detail satu ride.
class RideDetailState {
  const RideDetailState({
    this.isLoading = true,
    this.error,
    this.order,
  });

  final bool isLoading;
  final String? error;
  final RideOrder? order;

  RideDetailState copyWith({
    bool? isLoading,
    String? error,
    RideOrder? order,
  }) {
    return RideDetailState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      order: order ?? this.order,
    );
  }
}

/// Notifier detail satu ride (sekali fetch via GET /rides/{id}; retry via
/// tombol "Coba lagi"). Berbeda dari tracking — tidak ada polling.
class RideDetailNotifier extends StateNotifier<RideDetailState> {
  RideDetailNotifier(this._service, this._orderId)
      : super(const RideDetailState());

  final RideService _service;
  final String _orderId;

  Future<void> load() async {
    state = const RideDetailState();
    try {
      final order = await _service.getRideDetails(_orderId);
      state = RideDetailState(isLoading: false, order: order);
    } catch (_) {
      state = RideDetailState(
        isLoading: false,
        error: 'Gagal memuat detail perjalanan (jaringan/API). Pastikan server aktif.',
      );
    }
  }
}

final rideDetailProvider =
    StateNotifierProvider.family<RideDetailNotifier, RideDetailState, String>(
  (ref, orderId) {
    final notifier = RideDetailNotifier(ref.watch(rideServiceProvider), orderId);
    Future.microtask(notifier.load);
    return notifier;
  },
);

/// State riwayat ride (pagination, mirror pola FoodHistoryNotifier).
class RideHistoryState {
  const RideHistoryState({
    this.isLoading = false,
    this.error,
    this.orders = const [],
    this.hasMore = false,
  });

  final bool isLoading;
  final String? error;
  final List<RideOrder> orders;
  final bool hasMore;

  RideHistoryState copyWith({
    bool? isLoading,
    String? error,
    List<RideOrder>? orders,
    bool? hasMore,
  }) {
    return RideHistoryState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      orders: orders ?? this.orders,
      hasMore: hasMore ?? this.hasMore,
    );
  }
}

class RideHistoryNotifier extends StateNotifier<RideHistoryState> {
  RideHistoryNotifier(this._service) : super(const RideHistoryState());

  final RideService _service;
  int _page = 1;

  static const int _pageSize = 20;

  Future<void> loadFirst() async {
    _page = 1;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final result = await _service.getHistory(page: _page, pageSize: _pageSize);
      state = RideHistoryState(
        isLoading: false,
        orders: result.orders,
        hasMore: result.totalPages > _page,
      );
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat riwayat perjalanan.',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoading || !state.hasMore) return;
    _page++;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final result = await _service.getHistory(page: _page, pageSize: _pageSize);
      state = RideHistoryState(
        isLoading: false,
        orders: [...state.orders, ...result.orders],
        hasMore: result.totalPages > _page,
      );
    } catch (_) {
      state = state.copyWith(isLoading: false, error: 'Gagal memuat halaman berikut.');
    }
  }
}

final rideHistoryProvider =
    StateNotifierProvider<RideHistoryNotifier, RideHistoryState>((ref) {
  return RideHistoryNotifier(ref.watch(rideServiceProvider));
});