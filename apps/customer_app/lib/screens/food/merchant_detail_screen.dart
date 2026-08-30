import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/item_option.dart';
import '../../models/menu.dart';
import '../../models/menu_item.dart';
import '../../models/merchant.dart';
import '../../models/option_choice.dart';
import '../../providers/cart_provider.dart';
import '../../providers/merchant_provider.dart';

/// Merchant Detail & Menu (ROADMAP 3.8 B & HALAMAN.txt 4.2).
class MerchantDetailScreen extends ConsumerStatefulWidget {
  const MerchantDetailScreen({super.key});

  @override
  ConsumerState<MerchantDetailScreen> createState() => _MerchantDetailScreenState();
}

class _MerchantDetailScreenState extends ConsumerState<MerchantDetailScreen> {
  Merchant? _merchant;
  List<Menu> _menus = [];
  bool _loaded = false;

  @override
  Widget build(BuildContext context) {
    final raw = ModalRoute.of(context)?.settings.arguments;
    if (raw is Merchant) {
      _merchant = raw;
    }
    final merch = _merchant;
    if (merch == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Merchant')),
        body: const Center(child: Text('Argumen merchant tidak ditemukan.')),
      );
    }

    if (!_loaded) {
      _loaded = true;
      _loadMenus(merch.id);
    }

    final cart = ref.watch(cartProvider);

    return Scaffold(
      appBar: AppBar(title: Text(merch.name)),
      body: _body(merch),
      bottomNavigationBar: cart.itemCount > 0 ? _buildCartBar(cart) : null,
    );
  }

  Future<void> _loadMenus(String merchantId) async {
    final service = ref.read(merchantServiceProvider);
    try {
      var menus = await service.fetchMenus(merchantId);
      if (menus.isEmpty) {
        menus = await service.fetchItemsAsMenus(merchantId);
      }
      if (mounted) setState(() => _menus = menus);
    } catch (_) {
      if (mounted) setState(() => _menus = []);
    }
  }

  Widget _body(Merchant merch) {
    return Column(
      children: [
        _merchantHeader(merch),
        Expanded(child: _menuList()),
      ],
    );
  }

  Widget _merchantHeader(Merchant m) {
    final theme = Theme.of(context);
    return Container(
      width: double.infinity,
      color: theme.colorScheme.surfaceContainerLow,
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(m.name, style: theme.textTheme.titleLarge),
          const SizedBox(height: 6),
          Wrap(
            spacing: 12,
            runSpacing: 4,
            children: [
              _headerChip(Icons.star, m.rating.toStringAsFixed(1), Colors.amber),
              if (m.distanceKm > 0)
                _headerChip(Icons.place, '${m.distanceKm.toStringAsFixed(1)} km', null),
              _headerChip(
                Icons.schedule,
                m.isOpen ? 'Buka' : 'Tutup',
                m.isOpen ? Colors.green : Colors.red,
              ),
            ],
          ),
          if (m.address.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(
              m.address,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _headerChip(IconData icon, String label, Color? color) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 14, color: color ?? Colors.grey),
        const SizedBox(width: 2),
        Text(label, style: const TextStyle(fontSize: 12)),
      ],
    );
  }

  Widget _menuList() {
    if (_menus.isEmpty) {
      return const Center(child: Text('Menu kosong atau gagal dimuat.'));
    }
    return ListView(
      padding: const EdgeInsets.all(16),
      children: _menus.map((menu) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(
                menu.name.toUpperCase(),
                style: Theme.of(context).textTheme.titleSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: Theme.of(context).colorScheme.primary,
                    ),
              ),
            ),
            for (final item in menu.items) _itemCard(item),
          ],
        );
      }).toList(),
    );
  }

  Widget _itemCard(MenuItem item) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: item.isAvailable ? () => _showItemSheet(item) : null,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: Container(
                  width: 60,
                  height: 60,
                  color: theme.colorScheme.surfaceContainerHighest,
                  child: item.imageUrl != null && item.imageUrl!.isNotEmpty
                      ? Image.network(
                          item.imageUrl!,
                          fit: BoxFit.cover,
                          errorBuilder: (_, _, _) =>
                              const Icon(Icons.restaurant_menu),
                        )
                      : const Icon(Icons.restaurant_menu),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      item.name,
                      style: theme.textTheme.titleSmall,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (item.description.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        item.description,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            _formatRupiah(item.price),
                            style: theme.textTheme.titleSmall?.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                        if (item.isAvailable)
                          FilledButton.tonal(
                            onPressed: () => _showItemSheet(item),
                            child: const Text('+ Keranjang'),
                          )
                        else
                          const Text(
                            'Habis',
                            style: TextStyle(color: Colors.red, fontSize: 12),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showItemSheet(MenuItem item) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => _ItemSheet(item: item),
    );
  }

  Widget _buildCartBar(CartState cart) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: FilledButton.icon(
          style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(52)),
          onPressed: () => Navigator.of(context).pushNamed('/food-cart'),
          icon: const Icon(Icons.shopping_cart_outlined),
          label: Text(
            'Keranjang (${cart.itemCount}) · ${_formatRupiah(cart.subtotal)} →  Checkout',
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ),
    );
  }
}

/// Bottom sheet: detail item + pilihan opsi (SINGLE/MULTIPLE) + kuantitas.
class _ItemSheet extends ConsumerStatefulWidget {
  const _ItemSheet({required this.item});

  final MenuItem item;

  @override
  ConsumerState<_ItemSheet> createState() => _ItemSheetState();
}

class _ItemSheetState extends ConsumerState<_ItemSheet> {
  final Map<String, OptionChoice> _singleSelections = {};
  final Map<String, Set<String>> _multipleSelections = {};
  int _quantity = 1;

  int get _unitPrice {
    var p = widget.item.price;
    for (final c in _singleSelections.values) {
      p += c.priceAdjustment;
    }
    for (final group in widget.item.options) {
      final set = _multipleSelections[group.id];
      if (set == null) continue;
      for (final c in group.options.where((o) => set.contains(o.id))) {
        p += c.priceAdjustment;
      }
    }
    return p;
  }

  bool _isGroupAnswered(ItemOptionGroup group) {
    if (!group.isRequired) return true;
    if (group.isMultiple) return (_multipleSelections[group.id] ?? {}).isNotEmpty;
    return _singleSelections.containsKey(group.id);
  }

  void _addToCart() {
    for (final group in widget.item.options) {
      if (!_isGroupAnswered(group)) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Pilih opsi "${group.name}" terlebih dahulu.')),
        );
        return;
      }
    }

    final selections = <ItemSelection>[];
    for (final group in widget.item.options) {
      final single = _singleSelections[group.id];
      if (single != null) {
        selections.add(ItemSelection(group: group, choice: single));
      }
      final set = _multipleSelections[group.id] ?? {};
      for (final c in group.options.where((o) => set.contains(o.id))) {
        selections.add(ItemSelection(group: group, choice: c));
      }
    }

    ref.read(cartProvider.notifier).addItem(widget.item, selections, quantity: _quantity);
    Navigator.of(context).pop();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('${widget.item.name} ditambahkan ke keranjang.')),
    );
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: SingleChildScrollView(
        padding: EdgeInsets.only(
          left: 16,
          right: 16,
          top: 16,
          bottom: MediaQuery.of(context).viewInsets.bottom + 16,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    widget.item.name,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                ),
                IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close)),
              ],
            ),
            if (widget.item.description.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(widget.item.description, style: const TextStyle(color: Colors.grey)),
            ],
            const SizedBox(height: 8),
            Text(
              _formatRupiah(widget.item.price),
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            for (final group in widget.item.options)
              _OptionGroupWidget(
                group: group,
                singleSelections: _singleSelections,
                multipleSelections: _multipleSelections,
                onChanged: () => setState(() {}),
              ),
            const Divider(height: 24),
            Row(
              children: [
                const Text('Jumlah'),
                const Spacer(),
                IconButton(
                  onPressed: _quantity > 1
                      ? () => setState(() => _quantity--)
                      : null,
                  icon: const Icon(Icons.remove_circle_outline),
                ),
                Text('$_quantity'),
                IconButton(
                  onPressed: () => setState(() => _quantity++),
                  icon: const Icon(Icons.add_circle_outline),
                ),
              ],
            ),
            const SizedBox(height: 8),
            FilledButton.icon(
              style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(50)),
              onPressed: _addToCart,
              icon: const Icon(Icons.add_shopping_cart),
              label: Text(
                  'Tambah ke Keranjang · ${_formatRupiah(_unitPrice * _quantity)}'),
            ),
          ],
        ),
      ),
    );
  }
}

/// Widget grup opsi: radio untuk SINGLE, checkbox untuk MULTIPLE.
class _OptionGroupWidget extends StatelessWidget {
  const _OptionGroupWidget({
    required this.group,
    required this.singleSelections,
    required this.multipleSelections,
    required this.onChanged,
  });

  final ItemOptionGroup group;
  final Map<String, OptionChoice> singleSelections;
  final Map<String, Set<String>> multipleSelections;
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                group.name,
                style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.bold),
              ),
              if (group.isRequired) ...[
                const SizedBox(width: 4),
                const Text('*', style: TextStyle(color: Colors.red)),
              ],
            ],
          ),
          const SizedBox(height: 4),
          if (group.isMultiple)
            for (final choice in group.options)
              CheckboxListTile(
                dense: true,
                contentPadding: EdgeInsets.zero,
                title: Text('${choice.name}  (+${_formatRupiah(choice.priceAdjustment)})'),
                value: (multipleSelections[group.id] ?? {}).contains(choice.id),
                controlAffinity: ListTileControlAffinity.leading,
                onChanged: (v) {
                  final set = {...(multipleSelections[group.id] ?? {})};
                  if (v == true) {
                    set.add(choice.id);
                  } else {
                    set.remove(choice.id);
                  }
                  multipleSelections[group.id] = set;
                  onChanged();
                },
              )
          else
            RadioGroup<String>(
              groupValue: singleSelections[group.id]?.id,
              onChanged: (v) {
                if (v == null) return;
                for (final c in group.options) {
                  if (c.id == v) {
                    singleSelections[group.id] = c;
                    break;
                  }
                }
                onChanged();
              },
              child: Column(
                children: [
                  for (final choice in group.options)
                    RadioListTile<String>(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      title: Text(
                          '${choice.name}  (+${_formatRupiah(choice.priceAdjustment)})'),
                      value: choice.id,
                      controlAffinity: ListTileControlAffinity.leading,
                    ),
                ],
              ),
            ),
        ],
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
