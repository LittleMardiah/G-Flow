import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/item_option.dart';
import '../models/menu_item.dart';
import '../models/option_choice.dart';

/// Satu baris item di dalam cart: item + kuantitas + opsi terpilih.
class CartLine {
  const CartLine({
    required this.menuItem,
    required this.selectedOptions,
    this.quantity = 1,
  });

  final MenuItem menuItem;
  final List<ItemSelection> selectedOptions;
  final int quantity;

  int get unitPrice {
    var p = menuItem.price;
    for (final s in selectedOptions) {
      p += s.choice.priceAdjustment;
    }
    return p;
  }

  int get subtotal => unitPrice * quantity;

  CartLine copyWith({int? quantity}) {
    return CartLine(
      menuItem: menuItem,
      selectedOptions: selectedOptions,
      quantity: quantity ?? this.quantity,
    );
  }

  /// Kunci unik berdasar item id + opsi terpilih (versi yang beda opsi
  /// dianggap baris berbeda).
  String get key {
    final opts = selectedOptions.map((s) => s.choice.id).join('|');
    return '${menuItem.id}::$opts';
  }
}

/// Satu pilihan opsi per grup pada sebuah item di cart.
class ItemSelection {
  const ItemSelection({required this.group, required this.choice});

  final ItemOptionGroup group;
  final OptionChoice choice;
}

/// State cart: daftar baris, subtotal, delivery fee, diskon, voucher.
class CartState {
  const CartState({
    this.lines = const [],
    this.deliveryFee = 0,
    this.discountAmount = 0,
    this.voucherCode,
    this.merchantId,
  });

  final List<CartLine> lines;
  final int deliveryFee;
  final int discountAmount;
  final String? voucherCode;
  final String? merchantId;

  int get itemCount =>
      lines.fold(0, (sum, l) => sum + l.quantity);
  int get subtotal => lines.fold(0, (sum, l) => sum + l.subtotal);
  int get total => subtotal - discountAmount + deliveryFee;

  CartState copyWith({
    List<CartLine>? lines,
    int? deliveryFee,
    int? discountAmount,
    String? voucherCode,
    String? merchantId,
  }) {
    return CartState(
      lines: lines ?? this.lines,
      deliveryFee: deliveryFee ?? this.deliveryFee,
      discountAmount: discountAmount ?? this.discountAmount,
      voucherCode: voucherCode ?? this.voucherCode,
      merchantId: merchantId ?? this.merchantId,
    );
  }
}

class CartNotifier extends StateNotifier<CartState> {
  CartNotifier() : super(const CartState());

  void addItem(MenuItem item, List<ItemSelection> options, {int quantity = 1}) {
    final line = CartLine(
      menuItem: item,
      selectedOptions: options,
      quantity: quantity,
    );
    // Simpan merchant pertama kali item ditambahkan (keranjang diasumsikan
    // satu merchant — multi-merchant di luar scope MVP).
    final merchantId = state.merchantId ?? item.merchantId;
    final idx = state.lines.indexWhere((l) => l.key == line.key);
    if (idx >= 0) {
      final existing = state.lines[idx];
      state = state.copyWith(
        merchantId: merchantId,
        lines: [...state.lines]
          ..[idx] = existing.copyWith(quantity: existing.quantity + quantity),
      );
    } else {
      state = state.copyWith(
        merchantId: merchantId,
        lines: [...state.lines, line],
      );
    }
  }

  void removeItem(CartLine line) {
    state = state.copyWith(
      lines: state.lines.where((l) => l.key != line.key).toList(),
    );
  }

  void updateQuantity(CartLine line, int quantity) {
    if (quantity <= 0) {
      removeItem(line);
      return;
    }
    final idx = state.lines.indexWhere((l) => l.key == line.key);
    if (idx < 0) return;
    final lines = [...state.lines];
    lines[idx] = line.copyWith(quantity: quantity);
    state = state.copyWith(lines: lines);
  }

  void setDeliveryFee(int fee) => state = state.copyWith(deliveryFee: fee);

  void applyVoucher(String code, int discount) {
    state = state.copyWith(voucherCode: code, discountAmount: discount);
  }

  void removeVoucher() {
    state = state.copyWith(voucherCode: null, discountAmount: 0);
  }

  void clear() => state = const CartState();
}

final cartProvider = StateNotifierProvider<CartNotifier, CartState>((ref) {
  return CartNotifier();
});
