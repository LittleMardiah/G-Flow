import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/cart_provider.dart';

/// Food Cart Screen (ROADMAP 3.8 C & HALAMAN.txt 4.3).
class FoodCartScreen extends ConsumerWidget {
  const FoodCartScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cart = ref.watch(cartProvider);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: Text('Keranjang (${cart.itemCount})')),
      body: cart.lines.isEmpty
          ? const Center(child: Text('Keranjang kosong.'))
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                for (final line in cart.lines) _CartLineTile(line: line),
                const Divider(height: 28),
                _priceRow('Subtotal', cart.subtotal, theme),
                _priceRow('Delivery Fee', cart.deliveryFee, theme),
                if (cart.discountAmount > 0)
                  _priceRow('Diskon', -cart.discountAmount, theme,
                      color: Colors.green),
                const Divider(height: 16),
                _priceRow('Total', cart.total, theme, bold: true),
                const SizedBox(height: 8),
                _VoucherSection(),
              ],
            ),
      bottomNavigationBar: cart.lines.isEmpty
          ? null
          : SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: FilledButton.icon(
                  style:
                      FilledButton.styleFrom(minimumSize: const Size.fromHeight(52)),
                  onPressed: () =>
                      Navigator.of(context).pushNamed('/food-checkout'),
                  icon: const Icon(Icons.shopping_cart_checkout),
                  label: const Text('CONFIRM & CHECKOUT'),
                ),
              ),
            ),
    );
  }

  Widget _priceRow(String label, int value, ThemeData theme,
      {Color? color, bool bold = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: bold
                ? theme.textTheme.titleMedium
                : theme.textTheme.bodyMedium,
          ),
          Text(
            _formatRupiah(value),
            style: (bold ? theme.textTheme.titleMedium : theme.textTheme.bodyMedium)
                ?.copyWith(
              fontWeight: bold ? FontWeight.bold : FontWeight.normal,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}

class _CartLineTile extends ConsumerWidget {
  const _CartLineTile({required this.line});

  final CartLine line;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    line.menuItem.name,
                    style: theme.textTheme.titleSmall,
                  ),
                ),
                IconButton(
                  visualDensity: VisualDensity.compact,
                  icon: const Icon(Icons.close, color: Colors.red, size: 20),
                  onPressed: () =>
                      ref.read(cartProvider.notifier).removeItem(line),
                ),
              ],
            ),
            if (line.selectedOptions.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Text(
                  line.selectedOptions.map((s) => s.choice.name).join(', '),
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            Row(
              children: [
                Text(
                  _formatRupiah(line.unitPrice),
                  style: theme.textTheme.bodyMedium,
                ),
                const Spacer(),
                IconButton(
                  visualDensity: VisualDensity.compact,
                  onPressed: () => ref
                      .read(cartProvider.notifier)
                      .updateQuantity(line, line.quantity - 1),
                  icon: const Icon(Icons.remove_circle_outline),
                ),
                Text('${line.quantity}'),
                IconButton(
                  visualDensity: VisualDensity.compact,
                  onPressed: () => ref
                      .read(cartProvider.notifier)
                      .updateQuantity(line, line.quantity + 1),
                  icon: const Icon(Icons.add_circle_outline),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _VoucherSection extends ConsumerStatefulWidget {
  @override
  ConsumerState<_VoucherSection> createState() => _VoucherSectionState();
}

class _VoucherSectionState extends ConsumerState<_VoucherSection> {
  final TextEditingController _controller = TextEditingController();

  @override
  Widget build(BuildContext context) {
    final cart = ref.watch(cartProvider);
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: cart.voucherCode != null
            ? Row(
                children: [
                  const Icon(Icons.local_offer, color: Colors.green),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Voucher ${cart.voucherCode} · -${_formatRupiah(cart.discountAmount)}',
                      style: const TextStyle(color: Colors.green),
                    ),
                  ),
                  TextButton(
                    onPressed: () =>
                        ref.read(cartProvider.notifier).removeVoucher(),
                    child: const Text('Hapus'),
                  ),
                ],
              )
            : Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      decoration: const InputDecoration(
                        labelText: 'Kode voucher',
                        isDense: true,
                        border: OutlineInputBorder(),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  FilledButton.tonal(
                    onPressed: _apply,
                    child: const Text('Terapkan'),
                  ),
                ],
              ),
      ),
    );
  }

  void _apply() {
    final code = _controller.text.trim();
    if (code.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Masukkan kode voucher.')),
      );
      return;
    }
    // Demo: diskon flat 10% dari subtotal.
    final subtotal = ref.read(cartProvider).subtotal;
    final discount = (subtotal * 0.10).round();
    ref.read(cartProvider.notifier).applyVoucher(code, discount);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Voucher "$code" diterapkan.')),
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
