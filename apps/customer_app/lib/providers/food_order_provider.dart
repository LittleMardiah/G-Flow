import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/food_order.dart';
import '../services/food_order_service.dart';
import 'auth_provider.dart';

final foodOrderServiceProvider = Provider((ref) {
  return FoodOrderService(ref.watch(apiClientProvider));
});

/// DTO untuk create food order (dari cart).
class CreateFoodOrderInput {
  const CreateFoodOrderInput({
    required this.merchantId,
    required this.deliveryAddress,
    this.deliveryLat,
    this.deliveryLng,
    required this.paymentMethod,
    required this.items,
  });

  final String merchantId;
  final String deliveryAddress;
  final double? deliveryLat;
  final double? deliveryLng;
  final String paymentMethod;
  final List<Map<String, dynamic>> items;
}

/// State checkout food order.
class FoodOrderSubmitState {
  const FoodOrderSubmitState({
    this.isSubmitting = false,
    this.error,
    this.orderId,
  });

  final bool isSubmitting;
  final String? error;
  final String? orderId;

  FoodOrderSubmitState copyWith({
    bool? isSubmitting,
    String? error,
    String? orderId,
  }) {
    return FoodOrderSubmitState(
      isSubmitting: isSubmitting ?? this.isSubmitting,
      error: error ?? this.error,
      orderId: orderId ?? this.orderId,
    );
  }
}

class FoodOrderSubmitNotifier extends StateNotifier<FoodOrderSubmitState> {
  FoodOrderSubmitNotifier(this._service) : super(const FoodOrderSubmitState());

  final FoodOrderService _service;

  Future<String?> submit(CreateFoodOrderInput input) async {
    state = state.copyWith(isSubmitting: true, error: null);
    try {
      final order = await _service.createOrder(
        merchantId: input.merchantId,
        items: input.items,
        deliveryAddress: input.deliveryAddress,
        deliveryLat: input.deliveryLat,
        deliveryLng: input.deliveryLng,
        paymentMethod: input.paymentMethod,
      );
      state = state.copyWith(isSubmitting: false, orderId: order.id);
      return order.id;
    } catch (e) {
      state = state.copyWith(
        isSubmitting: false,
        error: 'Gagal membuat pesanan. Pastikan saldo/pembayaran valid & server aktif.',
      );
      return null;
    }
  }

  void reset() => state = const FoodOrderSubmitState();
}

final foodOrderSubmitProvider = StateNotifierProvider<FoodOrderSubmitNotifier, FoodOrderSubmitState>(
  (ref) => FoodOrderSubmitNotifier(ref.watch(foodOrderServiceProvider)),
);

/// Tracking food order (polling status).
class FoodTrackingState {
  const FoodTrackingState({
    this.isLoading = true,
    this.isMock = false,
    this.error,
    this.order,
  });

  final bool isLoading;
  final bool isMock;
  final String? error;
  final FoodOrder? order;

  FoodTrackingState copyWith({
    bool? isLoading,
    bool? isMock,
    String? error,
    FoodOrder? order,
  }) {
    return FoodTrackingState(
      isLoading: isLoading ?? this.isLoading,
      isMock: isMock ?? this.isMock,
      error: error ?? this.error,
      order: order ?? this.order,
    );
  }
}

class FoodTrackingNotifier extends StateNotifier<FoodTrackingState> {
  FoodTrackingNotifier(this._service, this._args)
      : super(FoodTrackingState(isLoading: true, isMock: kUseMockFoodTracking));

  final FoodOrderService _service;
  final FoodOrderTrackingArgs _args;

  Timer? _timer;
  int _tick = 0;
  bool _polling = false;

  static const Duration pollInterval = Duration(seconds: 5);

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

  Future<void> retry() => _fetch();

  Future<void> _fetch() async {
    if (!_polling) return;
    if (state.order?.isTerminal == true) {
      stopPolling();
      return;
    }

    if (kUseMockFoodTracking) {
      _applyMockStep();
      return;
    }

    try {
      final order = await _service.getOrder(_args.orderId);
      state = FoodTrackingState(isLoading: false, isMock: false, order: order);
      if (order.isTerminal) stopPolling();
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat status pesanan (jaringan/API). Pastikan server aktif.',
      );
    }
  }

  void _applyMockStep() {
    _tick++;
    final status = _mockStatusForTick(_tick);
    final order = FoodOrder(
      id: _args.orderId,
      merchantId: _args.merchantId,
      status: status,
      totalAmount: _args.totalAmount,
      paymentMethod: _args.paymentMethod,
      isMock: true,
    );
    state = FoodTrackingState(isLoading: false, isMock: true, order: order);
    if (order.isTerminal) stopPolling();
  }

  static String _mockStatusForTick(int tick) {
    if (tick <= 1) return 'CREATED';
    if (tick <= 3) return 'CONFIRMED';
    if (tick <= 6) return 'PREPARING';
    if (tick <= 8) return 'READY_FOR_PICKUP';
    if (tick <= 10) return 'PICKED_UP';
    if (tick <= 13) return 'IN_TRANSIT';
    return 'DELIVERED';
  }

  Future<bool> cancelOrder() async {
    if (state.order?.isTerminal == true) return false;
    try {
      if (!kUseMockFoodTracking) {
        await _service.updateStatus(_args.orderId, 'CANCELLED');
      } else {
        await Future<void>.delayed(const Duration(milliseconds: 700));
      }
      state = FoodTrackingState(
        isLoading: false,
        isMock: kUseMockFoodTracking,
        order: FoodOrder(
          id: _args.orderId,
          merchantId: _args.merchantId,
          status: 'CANCELLED',
          totalAmount: _args.totalAmount,
          paymentMethod: _args.paymentMethod,
          isMock: kUseMockFoodTracking,
        ),
      );
      stopPolling();
      return true;
    } catch (_) {
      state = state.copyWith(
        error: 'Gagal membatalkan pesanan. Periksa koneksi lalu coba lagi.',
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

final foodTrackingProvider =
    StateNotifierProvider.family<FoodTrackingNotifier, FoodTrackingState, FoodOrderTrackingArgs>(
  (ref, args) {
    final notifier = FoodTrackingNotifier(ref.watch(foodOrderServiceProvider), args);
    return notifier..start();
  },
);

/// Riwayat food order.
class FoodHistoryState {
  const FoodHistoryState({
    this.isLoading = false,
    this.error,
    this.orders = const [],
    this.hasMore = false,
  });

  final bool isLoading;
  final String? error;
  final List<FoodOrder> orders;
  final bool hasMore;

  FoodHistoryState copyWith({
    bool? isLoading,
    String? error,
    List<FoodOrder>? orders,
    bool? hasMore,
  }) {
    return FoodHistoryState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      orders: orders ?? this.orders,
      hasMore: hasMore ?? this.hasMore,
    );
  }
}

class FoodHistoryNotifier extends StateNotifier<FoodHistoryState> {
  FoodHistoryNotifier(this._service) : super(const FoodHistoryState());

  final FoodOrderService _service;
  int _page = 1;

  Future<void> loadFirst() async {
    _page = 1;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final orders = await _service.getHistory(page: _page, limit: 10);
      state = FoodHistoryState(
        isLoading: false,
        orders: orders,
        hasMore: orders.length >= 10,
      );
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat riwayat pesanan.',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoading || !state.hasMore) return;
    _page++;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final orders = await _service.getHistory(page: _page, limit: 10);
      state = FoodHistoryState(
        isLoading: false,
        orders: [...state.orders, ...orders],
        hasMore: orders.length >= 10,
      );
    } catch (_) {
      state = state.copyWith(isLoading: false, error: 'Gagal memuat halaman berikut.');
    }
  }
}

final foodHistoryProvider =
    StateNotifierProvider<FoodHistoryNotifier, FoodHistoryState>((ref) {
  return FoodHistoryNotifier(ref.watch(foodOrderServiceProvider));
});
