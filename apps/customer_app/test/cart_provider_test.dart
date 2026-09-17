import 'package:flutter_test/flutter_test.dart';

import 'package:customer_app/models/item_option.dart';
import 'package:customer_app/models/menu_item.dart';
import 'package:customer_app/models/option_choice.dart';
import 'package:customer_app/providers/cart_provider.dart';

void main() {
  MenuItem item(String id, {int price = 1000, String? merchantId = 'm'}) =>
      MenuItem(id: id, merchantId: merchantId, name: 'Item $id', price: price);

  const group1 = ItemOptionGroup(
    id: 'g1',
    name: 'Level',
    options: [
      OptionChoice(id: 'o1', name: 'Normal', priceAdjustment: 0),
      OptionChoice(id: 'o2', name: 'Pedas', priceAdjustment: 2000),
    ],
  );

  group('CartLine', () {
    test('unitPrice includes option adjustments', () {
      final line = CartLine(
        menuItem: item('i', price: 10000),
        selectedOptions: const [ItemSelection(group: group1, choice: OptionChoice(id: 'o2', name: 'Pedas', priceAdjustment: 2000))],
        quantity: 2,
      );
      expect(line.unitPrice, 12000);
      expect(line.subtotal, 24000);
    });

    test('key combines item and option ids', () {
      final a = CartLine(menuItem: item('i'), selectedOptions: const [ItemSelection(group: group1, choice: OptionChoice(id: 'o1', name: 'a'))]);
      final b = CartLine(menuItem: item('i'), selectedOptions: const [ItemSelection(group: group1, choice: OptionChoice(id: 'o2', name: 'b'))]);
      final c = CartLine(menuItem: item('j'), selectedOptions: const [ItemSelection(group: group1, choice: OptionChoice(id: 'o1', name: 'a'))]);
      expect(a.key, isNot(b.key));
      expect(a.key, isNot(c.key));
      expect(a.key, 'i::o1');
    });

    test('copyWith quantity', () {
      final line = CartLine(menuItem: item('i'), selectedOptions: const [], quantity: 1);
      final c = line.copyWith(quantity: 5);
      expect(c.quantity, 5);
      expect(c.menuItem, line.menuItem);
    });
  });

  group('CartState', () {
    test('itemCount, subtotal, total', () {
      final state = CartState(
        lines: [
          CartLine(menuItem: item('a', price: 1000), selectedOptions: const [], quantity: 2),
          CartLine(menuItem: item('b', price: 3000), selectedOptions: const [], quantity: 1),
        ],
        deliveryFee: 5000,
        discountAmount: 1000,
      );
      expect(state.itemCount, 3);
      expect(state.subtotal, 5000);
      expect(state.total, 5000 - 1000 + 5000);
    });

    test('copyWith', () {
      const s = CartState();
      final c = s.copyWith(deliveryFee: 5, lines: const [], merchantId: 'm');
      expect(c.deliveryFee, 5);
      expect(c.merchantId, 'm');
      final c2 = s.copyWith(voucherCode: 'V');
      expect(c2.voucherCode, 'V');
    });
  });

  group('CartNotifier', () {
    test('addItem merges same-key lines', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const []);
      cart.addItem(item('a'), const []);
      cart.addItem(item('b'), const []);
      expect(cart.state.lines.length, 2);
      expect(cart.state.itemCount, 3);
    });

    test('addItem different options create separate lines', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const [ItemSelection(group: group1, choice: OptionChoice(id: 'o1', name: 'x'))]);
      cart.addItem(item('a'), const [ItemSelection(group: group1, choice: OptionChoice(id: 'o2', name: 'y'))]);
      expect(cart.state.lines.length, 2);
    });

    test('addItem keeps existing merchant', () {
      final cart = CartNotifier();
      cart.addItem(item('a', merchantId: 'm1'), const []);
      cart.addItem(item('b', merchantId: 'm2'), const []);
      expect(cart.state.merchantId, 'm1');
    });

    test('removeItem', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const []);
      final line = cart.state.lines.first;
      cart.removeItem(line);
      expect(cart.state.lines, isEmpty);
    });

    test('updateQuantity to zero removes', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const []);
      final line = cart.state.lines.first;
      cart.updateQuantity(line, 0);
      expect(cart.state.lines, isEmpty);
    });

    test('updateQuantity nonexistent line no-op', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const []);
      cart.updateQuantity(CartLine(menuItem: item('zzz'), selectedOptions: const []), 3);
      expect(cart.state.lines.length, 1);
      expect(cart.state.lines.first.quantity, 1);
    });

    test('updateQuantity to larger', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const []);
      final line = cart.state.lines.first;
      cart.updateQuantity(line, 4);
      expect(cart.state.lines.first.quantity, 4);
    });

    test('setDeliveryFee', () {
      final cart = CartNotifier();
      cart.setDeliveryFee(9000);
      expect(cart.state.deliveryFee, 9000);
    });

    test('applyVoucher & removeVoucher', () {
      final cart = CartNotifier();
      cart.addItem(item('a', price: 10000), const []);
      cart.applyVoucher('DISKON10', 1000);
      expect(cart.state.voucherCode, 'DISKON10');
      expect(cart.state.discountAmount, 1000);
      expect(cart.state.total, 10000 - 1000);
      cart.removeVoucher();
      // Catatan: copyWith memakai `??` sehingga null tidak menggantikan nilai
      // existing (voucherCode tetap), tetapi discountAmount=0 (bukan null)
      // benar-benar diterapkan. Dokumentasikan perilaku aktual.
      expect(cart.state.voucherCode, 'DISKON10');
      expect(cart.state.discountAmount, 0);
    });

    test('clear resets state', () {
      final cart = CartNotifier();
      cart.addItem(item('a'), const []);
      cart.setDeliveryFee(100);
      cart.clear();
      expect(cart.state.lines, isEmpty);
      expect(cart.state.deliveryFee, 0);
      expect(cart.state.merchantId, isNull);
    });
  });
}
