import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/food_order.dart';
import '../../providers/cart_provider.dart';
import '../../providers/food_order_provider.dart';

/// Food Order Checkout (ROADMAP 3.8 D & HALAMAN.txt 4.3).
class FoodCheckoutScreen extends ConsumerStatefulWidget {
  const FoodCheckoutScreen({super.key});

  @override
  ConsumerState<FoodCheckoutScreen> createState() => _FoodCheckoutScreenState();
}

class _FoodCheckoutScreenState extends ConsumerState<FoodCheckoutScreen> {
  final TextEditingController _addressController = TextEditingController();
  String _paymentMethod = 'WALLET';

  @override
  Widget build(BuildContext context) {
    final cart = ref.watch(cartProvider);
    final submit = ref.watch(foodOrderSubmitProvider);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Checkout')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text('Alamat Pengiriman', style: theme.textTheme.titleSmall),
          const SizedBox(height: 8),
          TextField(
            controller: _addressController,
            decoration: const InputDecoration(
              hintText: 'Jalan, No, Kelurahan, Kota',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.home_outlined),
            ),
          ),
          const Divider(height: 28),
          Text('Metode Pembayaran', style: theme.textTheme.titleSmall),
          const SizedBox(height: 8),
          RadioGroup<String>(
            groupValue: _paymentMethod,
            onChanged: (v) => setState(() => _paymentMethod = v!),
            child: Column(
              children: [
                RadioListTile<String>(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: const Text('PayPulse Wallet'),
                  value: 'WALLET',
                ),
                RadioListTile<String>(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: const Text('Cash on Delivery'),
                  value: 'CASH',
                ),
              ],
            ),
          ),
          const Divider(height: 28),
          Text('Ringkasan Pesanan', style: theme.textTheme.titleSmall),
          const SizedBox(height: 8),
          for (final line in cart.lines)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Text(
                      '${line.quantity}x ${line.menuItem.name}',
                      style: theme.textTheme.bodyMedium,
                    ),
                  ),
                  Text(
                    _formatRupiah(line.subtotal),
                    style: theme.textTheme.bodyMedium,
                  ),
                ],
              ),
            ),
          const Divider(),
          _row('Subtotal', cart.subtotal, theme),
          _row('Delivery Fee', cart.deliveryFee, theme),
          if (cart.discountAmount > 0)
            _row('Diskon', -cart.discountAmount, theme,
                color: Colors.green),
          const Divider(),
          _row('Total', cart.total, theme, bold: true),
          if (submit.error != null)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(
                submit.error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: FilledButton.icon(
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(52)),
            onPressed: (submit.isSubmitting || cart.lines.isEmpty || cart.merchantId == null)
                ? null
                : () => _placeOrder(cart, submit.isSubmitting),
            icon: submit.isSubmitting
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.assignment_turned_in_outlined),
            label: Text(
                submit.isSubmitting ? 'Memproses…' : 'PLACE ORDER · ${_formatRupiah(cart.total)}'),
          ),
        ),
      ),
    );
  }

  Widget _row(String label, int value, ThemeData theme,
      {Color? color, bool bold = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label,
              style: bold
                  ? theme.textTheme.titleMedium
                  : theme.textTheme.bodyMedium),
          Text(_formatRupiah(value),
              style: (bold
                      ? theme.textTheme.titleMedium
                      : theme.textTheme.bodyMedium)
                  ?.copyWith(
                fontWeight: bold ? FontWeight.bold : FontWeight.normal,
                color: color,
              )),
        ],
      ),
    );
  }

  Future<void> _placeOrder(CartState cart, bool busy) async {
    final address = _addressController.text.trim();
    if (address.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Masukkan alamat pengiriman.')),
      );
      return;
    }

    final notifier = ref.read(foodOrderSubmitProvider.notifier);
    final items = cart.lines.map((line) {
      return {
        'item_id': line.menuItem.id,
        'quantity': line.quantity,
        'selected_options': line.selectedOptions
            .map((s) => {'option_id': s.choice.id, 'selected_value': s.choice.name})
            .toList(),
      };
    }).toList();

    final orderId = await notifier.submit(CreateFoodOrderInput(
      merchantId: cart.merchantId!,
      deliveryAddress: address,
      paymentMethod: _paymentMethod,
      items: items,
    ));

    if (orderId == null || !mounted) return;

    ref.read(cartProvider.notifier).clear();
    Navigator.of(context).pushReplacementNamed(
      '/food-tracking',
      arguments: FoodOrderTrackingArgs(
        orderId: orderId,
        merchantId: cart.merchantId,
        totalAmount: cart.total,
        paymentMethod: _paymentMethod,
      ),
    );
  }
}

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}
