import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/send_order.dart';
import '../services/send_order_service.dart';
import 'auth_provider.dart';

final sendOrderServiceProvider = Provider((ref) {
  return SendOrderService(ref.watch(apiClientProvider));
});

/// DTO create send order (mengikuti body POST /send-orders backend —
/// internal/send/handler.go & service.go).
class CreateSendOrderInput {
  const CreateSendOrderInput({
    required this.packageType,
    required this.stops,
    required this.paymentMethod,
    this.pickupAddress = '',
    this.pickupLat = 0,
    this.pickupLng = 0,
    this.weightKg = 0,
    this.declaredValue = 0,
  });

  final String packageType;
  final List<Map<String, dynamic>> stops;
  final String paymentMethod;
  final String pickupAddress;
  final double pickupLat;
  final double pickupLng;
  final double weightKg;
  final double declaredValue;
}

/// State submit send order.
class SendOrderSubmitState {
  const SendOrderSubmitState({
    this.isSubmitting = false,
    this.error,
    this.orderId,
  });

  final bool isSubmitting;
  final String? error;
  final String? orderId;

  SendOrderSubmitState copyWith({
    bool? isSubmitting,
    String? error,
    String? orderId,
  }) {
    return SendOrderSubmitState(
      isSubmitting: isSubmitting ?? this.isSubmitting,
      error: error ?? this.error,
      orderId: orderId ?? this.orderId,
    );
  }
}

class SendOrderSubmitNotifier extends StateNotifier<SendOrderSubmitState> {
  SendOrderSubmitNotifier(this._service) : super(const SendOrderSubmitState());

  final SendOrderService _service;

  Future<String?> submit(CreateSendOrderInput input) async {
    state = state.copyWith(isSubmitting: true, error: null);
    try {
      final order = await _service.createOrder(
        packageType: input.packageType,
        stops: input.stops,
        paymentMethod: input.paymentMethod,
        pickupAddress: input.pickupAddress,
        pickupLat: input.pickupLat,
        pickupLng: input.pickupLng,
        weightKg: input.weightKg,
        declaredValue: input.declaredValue,
      );
      state = state.copyWith(isSubmitting: false, orderId: order.id);
      return order.id;
    } catch (_) {
      state = state.copyWith(
        isSubmitting: false,
        error: 'Gagal membuat pesanan kirim. Pastikan saldo & server aktif.',
      );
      return null;
    }
  }

  void reset() => state = const SendOrderSubmitState();
}

final sendOrderSubmitProvider = StateNotifierProvider<SendOrderSubmitNotifier, SendOrderSubmitState>(
  (ref) => SendOrderSubmitNotifier(ref.watch(sendOrderServiceProvider)),
);

/// Tracking send order (polling).
class SendTrackingState {
  const SendTrackingState({
    this.isLoading = true,
    this.isMock = false,
    this.error,
    this.order,
  });

  final bool isLoading;
  final bool isMock;
  final String? error;
  final SendOrder? order;

  SendTrackingState copyWith({
    bool? isLoading,
    bool? isMock,
    String? error,
    SendOrder? order,
  }) {
    return SendTrackingState(
      isLoading: isLoading ?? this.isLoading,
      isMock: isMock ?? this.isMock,
      error: error ?? this.error,
      order: order ?? this.order,
    );
  }
}

class SendTrackingNotifier extends StateNotifier<SendTrackingState> {
  SendTrackingNotifier(this._service, this._args)
      : super(SendTrackingState(isLoading: true, isMock: kUseMockSendTracking));

  final SendOrderService _service;
  final SendOrderTrackingArgs _args;

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

    if (kUseMockSendTracking) {
      _applyMockStep();
      return;
    }

    try {
      final order = await _service.getOrder(_args.orderId);
      state = SendTrackingState(isLoading: false, isMock: false, order: order);
      if (order.isTerminal) stopPolling();
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat status pengiriman (jaringan/API). Pastikan server aktif.',
      );
    }
  }

  void _applyMockStep() {
    _tick++;
    final status = _mockStatusForTick(_tick);
    final order = SendOrder(
      id: _args.orderId,
      status: status,
      packageType: _args.packageType,
      totalAmount: _args.totalAmount,
      paymentMethod: _args.paymentMethod,
      isMock: true,
    );
    state = SendTrackingState(isLoading: false, isMock: true, order: order);
    if (order.isTerminal) stopPolling();
  }

  static String _mockStatusForTick(int tick) {
    if (tick <= 1) return 'SEARCHING_DRIVER';
    if (tick <= 3) return 'DRIVER_ASSIGNED';
    if (tick <= 5) return 'PICKED_UP';
    if (tick <= 8) return 'IN_TRANSIT';
    return 'DELIVERED';
  }

  Future<bool> cancelOrder() async {
    if (state.order?.isTerminal == true) return false;
    try {
      if (!kUseMockSendTracking) {
        await _service.updateStatus(_args.orderId, 'CANCELLED');
      } else {
        await Future<void>.delayed(const Duration(milliseconds: 700));
      }
      state = SendTrackingState(
        isLoading: false,
        isMock: kUseMockSendTracking,
        order: SendOrder(
          id: _args.orderId,
          status: 'CANCELLED',
          packageType: _args.packageType,
          totalAmount: _args.totalAmount,
          paymentMethod: _args.paymentMethod,
          isMock: kUseMockSendTracking,
        ),
      );
      stopPolling();
      return true;
    } catch (_) {
      state = state.copyWith(
        error: 'Gagal membatalkan pengiriman. Periksa koneksi lalu coba lagi.',
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

final sendTrackingProvider =
    StateNotifierProvider.family<SendTrackingNotifier, SendTrackingState, SendOrderTrackingArgs>(
  (ref, args) {
    final notifier = SendTrackingNotifier(ref.watch(sendOrderServiceProvider), args);
    return notifier..start();
  },
);

/// Riwayat send order.
class SendHistoryState {
  const SendHistoryState({
    this.isLoading = false,
    this.error,
    this.orders = const [],
    this.hasMore = false,
  });

  final bool isLoading;
  final String? error;
  final List<SendOrder> orders;
  final bool hasMore;

  SendHistoryState copyWith({
    bool? isLoading,
    String? error,
    List<SendOrder>? orders,
    bool? hasMore,
  }) {
    return SendHistoryState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      orders: orders ?? this.orders,
      hasMore: hasMore ?? this.hasMore,
    );
  }
}

class SendHistoryNotifier extends StateNotifier<SendHistoryState> {
  SendHistoryNotifier(this._service) : super(const SendHistoryState());

  final SendOrderService _service;
  int _page = 1;

  Future<void> loadFirst() async {
    _page = 1;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final orders = await _service.getHistory(page: _page, limit: 10);
      state = SendHistoryState(
        isLoading: false,
        orders: orders,
        hasMore: orders.length >= 10,
      );
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat riwayat pengiriman.',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoading || !state.hasMore) return;
    _page++;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final orders = await _service.getHistory(page: _page, limit: 10);
      state = SendHistoryState(
        isLoading: false,
        orders: [...state.orders, ...orders],
        hasMore: orders.length >= 10,
      );
    } catch (_) {
      state = state.copyWith(isLoading: false, error: 'Gagal memuat halaman berikut.');
    }
  }
}

final sendHistoryProvider =
    StateNotifierProvider<SendHistoryNotifier, SendHistoryState>((ref) {
  return SendHistoryNotifier(ref.watch(sendOrderServiceProvider));
});
