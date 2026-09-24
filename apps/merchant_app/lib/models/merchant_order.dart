class MerchantOrder {
  const MerchantOrder({
    required this.id,
    this.customerId,
    this.merchantId,
    this.driverId,
    this.status = '',
    this.merchantStatus = '',
    this.items = const [],
    this.itemSubtotal = 0,
    this.deliveryFee = 0,
    this.discountAmount = 0,
    this.totalAmount = 0,
    this.paymentMethod = 'WALLET',
    this.deliveryAddress = '',
    this.specialInstructions,
    this.merchantNotes,
    this.createdAt,
    this.confirmedAt,
    this.pickupAt,
    this.deliveredAt,
    this.isSettled = false,
  });

  final String id;
  final String? customerId;
  final String? merchantId;
  final String? driverId;
  final String status;
  final String merchantStatus;
  final List<MerchantOrderItem> items;
  final int itemSubtotal;
  final int deliveryFee;
  final int discountAmount;
  final int totalAmount;
  final String paymentMethod;
  final String deliveryAddress;
  final String? specialInstructions;
  final String? merchantNotes;
  final DateTime? createdAt;
  final DateTime? confirmedAt;
  final DateTime? pickupAt;
  final DateTime? deliveredAt;
  final bool isSettled;

  String get idShort =>
      id.length > 8 ? '#${id.substring(0, 8).toUpperCase()}' : '#${id.toUpperCase()}';

  String get displayStatus {
    final st = status.toUpperCase();
    final ms = merchantStatus.toUpperCase();
    // Status order terminal mendahului merchant_status: order yang di-reject
    // merchant punya merchant_status 'WAITING' (DB CHECK constraint menolak
    // 'CANCELLED'), tapi harus tampil CANCELLED + tanpa aksi confirm/reject.
    if (const {'CANCELLED', 'DELIVERED', 'SETTLED'}.contains(st)) return st;
    if (ms.isNotEmpty) return ms;
    switch (st) {
      case '':
      case 'CREATED':
        return 'WAITING';
      case 'READY_FOR_PICKUP':
        return 'READY';
      default:
        return st;
    }
  }

  int get itemCount =>
      items.fold(0, (sum, it) => sum + it.quantity);

  bool get isWaiting => displayStatus == 'WAITING';
  bool get isConfirmed => displayStatus == 'CONFIRMED';
  bool get isPreparing => displayStatus == 'PREPARING';
  bool get isReady => displayStatus == 'READY';
  bool get isCancelled => displayStatus == 'CANCELLED';

  bool get isTerminal => const {'DELIVERED', 'SETTLED', 'CANCELLED'}.contains(displayStatus);

  List<MerchantOrderAction> get nextActions {
    if (displayStatus == 'WAITING') {
      return const [
        MerchantOrderAction(label: 'Confirm', targetStatus: 'CONFIRMED'),
        MerchantOrderAction(label: 'Reject', targetStatus: 'CANCELLED'),
      ];
    }
    if (displayStatus == 'CONFIRMED') {
      return const [
        MerchantOrderAction(label: 'Start Preparing', targetStatus: 'PREPARING'),
      ];
    }
    if (displayStatus == 'PREPARING') {
      return const [
        MerchantOrderAction(label: 'Ready for Pickup', targetStatus: 'READY_FOR_PICKUP'),
      ];
    }
    return const [];
  }

  MerchantOrder copyWith({String? status, String? merchantStatus, String? merchantNotes}) {
    return MerchantOrder(
      id: id,
      customerId: customerId,
      merchantId: merchantId,
      driverId: driverId,
      status: status ?? this.status,
      merchantStatus: merchantStatus ?? this.merchantStatus,
      items: items,
      itemSubtotal: itemSubtotal,
      deliveryFee: deliveryFee,
      discountAmount: discountAmount,
      totalAmount: totalAmount,
      paymentMethod: paymentMethod,
      deliveryAddress: deliveryAddress,
      specialInstructions: specialInstructions,
      merchantNotes: merchantNotes ?? this.merchantNotes,
      createdAt: createdAt,
      confirmedAt: confirmedAt,
      pickupAt: pickupAt,
      deliveredAt: deliveredAt,
      isSettled: isSettled,
    );
  }

  factory MerchantOrder.fromJson(Map<String, dynamic> j) {
    return MerchantOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      customerId: _str(j['customer_id']),
      merchantId: _str(j['merchant_id']),
      driverId: _str(j['driver_id']),
      status: _str(j['status']) ?? '',
      merchantStatus: _str(j['merchant_status']) ?? '',
      items: _list(j['items']).map(MerchantOrderItem.fromJson).toList(),
      itemSubtotal: _int(j['item_subtotal']),
      deliveryFee: _int(j['delivery_fee']),
      discountAmount: _int(j['discount_amount']),
      totalAmount: _int(j['total_amount']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      deliveryAddress: _str(j['delivery_address']) ?? '',
      specialInstructions: _str(j['special_instructions']),
      merchantNotes: _str(j['merchant_notes']),
      createdAt: _date(j['created_at']),
      confirmedAt: _date(j['confirmed_at']),
      pickupAt: _date(j['pickup_at']),
      deliveredAt: _date(j['delivered_at']),
      isSettled: j['is_settled'] == true,
    );
  }

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

class MerchantOrderAction {
  const MerchantOrderAction({required this.label, required this.targetStatus});

  final String label;
  final String targetStatus;
}

class MerchantOrderItem {
  const MerchantOrderItem({
    this.orderItemId,
    required this.itemName,
    this.quantity = 1,
    this.unitPrice = 0,
    this.subtotal = 0,
    this.selectedOptions = const [],
  });

  final String? orderItemId;
  final String itemName;
  final int quantity;
  final int unitPrice;
  final int subtotal;
  final List<String> selectedOptions;

  factory MerchantOrderItem.fromJson(Map<String, dynamic> j) {
    return MerchantOrderItem(
      orderItemId: _str(j['order_item_id']),
      itemName: _str(j['item_name'] ?? j['name']) ?? '',
      quantity: _int(j['quantity']),
      unitPrice: _int(j['unit_price'] ?? j['item_price'] ?? j['price']),
      subtotal: _int(j['subtotal']),
      selectedOptions: _splitOptions(j['options']),
    );
  }

  static List<String> _splitOptions(Object? v) {
    if (v is List) return v.map((e) => e.toString()).toList();
    final s = _str(v);
    if (s == null || s.isEmpty) return const [];
    return s.split(',').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}